package controllers

import (
	"dev-flows-api-golang/models"
	"github.com/golang/glog"
	"encoding/json"
	"gopkg.in/robfig/cron.v2"
	"dev-flows-api-golang/modules/client"
	"strings"
	"time"
	"sync"
	"fmt"
)

const (
	RepoTypeSVN = "SVN"
	RepoTypeGIT = "GIT"
)

var EnnCrontab *EnnFlowCrontab

func init() {
	EnnCrontab = NewEnnFlowCrontab()
	EnnCrontab.Start()
	go Crontabv2()
}

func Crontabv2() {

	method := "Crontab"

	models.NewCiCrontab().DeleteAllCiCrontab()

	stage := &models.CiStages{}
	stageInfos, total, err := stage.FindAllStage()

	if err != nil || total == 0 {
		glog.Errorf("%s Stages cannot be found %v\n", method, err)
	}

	for _, stageInfo := range stageInfos {

		if stageInfo.CiConfig != "" {
			glog.Infof("%s ci config info %s\n", method, stageInfo.CiConfig)

			var ciConfig models.CiConfig
			err = json.Unmarshal([]byte(stageInfo.CiConfig), &ciConfig)
			if err != nil {
				glog.Errorf("method ERROR:%v \n", err)
				continue
			}
			//repoType:SVN GIT
			if ciConfig.Crontab.Enabled == 1 && stageInfo.CiEnabled == 1 {
				ciFlow, _ := models.NewCiFlows().FindFlowByIdCrab(stageInfo.FlowId)
				EnnCrontab.RunCrontab(ciFlow, ciConfig.Crontab.CrontabTime, ciConfig.Crontab.RepoType, ciConfig.Crontab.Branch)

			}

			continue
		}

		continue
	}

}

type EnnFlowCrontab struct {
	Mutex   sync.RWMutex
	Crontab *cron.Cron
	Ids     map[string]cron.EntryID
	Id      int
}

func NewEnnFlowCrontab() *EnnFlowCrontab {
	return &EnnFlowCrontab{
		Crontab: cron.New(),
		Ids:     make(map[string]cron.EntryID, 0),
		Id:      1,
	}
}

func (ennCrontab *EnnFlowCrontab) Stop() {
	ennCrontab.Crontab.Stop()

}

func (ennCrontab *EnnFlowCrontab) Remove(id cron.EntryID) {
	ennCrontab.Crontab.Remove(id)

}

func (ennCrontab *EnnFlowCrontab) IdIncrease() {
	ennCrontab.Id++
}

func (ennCrontab *EnnFlowCrontab) AddIdToMap(flowId string, id cron.EntryID) {
	ennCrontab.Mutex.Lock()
	defer ennCrontab.Mutex.Unlock()
	ennCrontab.Ids[flowId] = id

}

func (ennCrontab *EnnFlowCrontab) Start() {
	ennCrontab.Crontab.Start()
}

func (ennCrontab *EnnFlowCrontab) Exist(flowId string) bool {
	ennCrontab.Mutex.Lock()
	defer ennCrontab.Mutex.Unlock()

	_, ok := ennCrontab.Ids[flowId]

	con, err := models.NewCiCrontab().FindCiCrontabByFlowId(flowId)
	if err != nil {
		glog.Errorf("", err)
	}

	return ok && con.Enabled == 1

}

func (ennCrontab *EnnFlowCrontab) DeleteIdToMap(flowId string) {
	ennCrontab.Mutex.Lock()
	defer ennCrontab.Mutex.Unlock()

	if _, ok := ennCrontab.Ids[flowId]; ok {
		delete(ennCrontab.Ids, flowId)
	}

}

func (ennCrontab *EnnFlowCrontab) GetCrontabId(flowId string) cron.EntryID {
	ennCrontab.Mutex.Lock()
	defer ennCrontab.Mutex.Unlock()

	return ennCrontab.Ids[flowId]

}

func (ennCrontab *EnnFlowCrontab) RunCrontab(ciFlow models.CiFlows, doCrontabTime time.Time, repoType, branch string) {

	DoCrontabTime := doCrontabTime.Format("05 04 15 * * *")

	glog.Infof("%s\n", doCrontabTime)

	crontabInfo := NewCrontabInfo(ciFlow, doCrontabTime, repoType, branch, ennCrontab.Id)

	id, _ := ennCrontab.Crontab.AddFunc(DoCrontabTime, crontabInfo.Run, crontabInfo.Id)

	ennCrontab.AddIdToMap(ciFlow.FlowId, id)

	ennCrontab.IdIncrease()
	var ciCon models.CiCrontab
	ciCon.CrontabId = ennCrontab.Id
	ciCon.Enabled = 1
	ciCon.FlowId = ciFlow.FlowId
	ciCon.DoCrontabTime = doCrontabTime
	_, err := models.NewCiCrontab().CreateOneCiCrontab(ciCon)
	if err != nil {
		glog.Errorf("NewCiCrontab.CreateOneCiCrontab failed:%v\n")

	}
	glog.Infof("======>>id=%d\n", id)
}

type CrontabInfo struct {
	EnnFlow       EnnFlow
	Id            int
	DoCrontabTime time.Time
}

func NewCrontabInfo(ciFlow models.CiFlows, doCrontabTime time.Time, repoType, branch string, id int) CrontabInfo {
	CodeBranch := ""

	if repoType == RepoTypeGIT {
		CodeBranch = branch
	}

	flow := EnnFlow{
		FlowId:        ciFlow.FlowId,
		StageId:       "",
		CodeBranch:    CodeBranch,
		LoginUserName: ciFlow.Owner,
		Namespace:     ciFlow.Namespace,
		UserNamespace: ciFlow.Namespace,
	}

	crontabInfo := CrontabInfo{
		EnnFlow:       flow,
		Id:            id,
		DoCrontabTime: doCrontabTime,
	}
	return crontabInfo
}

func (ennCrontab *CrontabInfo) Run() {

	//不传event参数 event 是代码分支
	imageBuild := models.NewImageBuilder(client.ClusterID)
	stagequeue := NewStageQueueNew(ennCrontab.EnnFlow, "", ennCrontab.EnnFlow.Namespace, ennCrontab.EnnFlow.LoginUserName,
		ennCrontab.EnnFlow.FlowId, imageBuild)

	stagequeue.Namespace = ennCrontab.EnnFlow.LoginUserName

	if stagequeue != nil {
		//判断是否该EnnFlow当前有执行中
		err := stagequeue.CheckIfBuiding(ennCrontab.EnnFlow.FlowId)
		if err != nil {
			if strings.Contains(fmt.Sprintf("%s", err), "该EnnFlow已有任务在执行,请等待执行完再试") {
				glog.Infof("该EnnFlow已有任务在执行:%s\n", ennCrontab.EnnFlow.FlowId)
				return
			} else {
				return
			}
		}
		//开始执行 把执行日志插入到数据库
		stagequeue.InsertLog()
		go stagequeue.Run()
	}
}
