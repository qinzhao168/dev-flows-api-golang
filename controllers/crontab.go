package controllers

import (
	"dev-flows-api-golang/models"
	"github.com/golang/glog"
	"encoding/json"
	"github.com/mileusna/crontab"
	"dev-flows-api-golang/modules/client"
	"strings"
	"time"
	"fmt"
)

const (
	RepoTypeSVN = "SVN"
	RepoTypeGIT = "GIT"
)

func init() {
	go Crontab()
}

func Crontab() {

	method := "Crontab"

	stage := &models.CiStages{}
	stageInfos, total, err := stage.FindAllStage()

	if err != nil || total == 0 {
		glog.Errorf("%s Stages cannot be found %v\n", method, err)
	}

	for _, stageInfo := range stageInfos {

		if stageInfo.CiConfig != "" {
			glog.Infof("%s ci config info %s\n", method, stage.CiConfig)

			var ciConfig models.CiConfig
			err = json.Unmarshal([]byte(stageInfo.CiConfig), &ciConfig)
			if err != nil {
				glog.Errorf("method ERROR:%v \n", err)
				continue
			}

			//repoType:SVN GIT
			if ciConfig.Crontab.Enabled == 1 && stageInfo.CiEnabled == 1 {
				ciFlow, _ := models.NewCiFlows().FindFlowByIdCrab(stageInfo.FlowId)
				EnnFlowCrontab := NewEnnFlowCrontab(ciFlow, ciConfig.Crontab.CrontabTime,
					ciConfig.Crontab.RepoType, ciConfig.Crontab.Branch)
				EnnFlowCrontab.RunCrontab()
			}

			continue
		}

		continue
	}

}

type EnnFlowCrontab struct {
	DoCrontabTime time.Time
	EnnFlow       EnnFlow
}

func NewEnnFlowCrontab(ciFlow models.CiFlows, doCrontabTime time.Time, repoType, branch string) *EnnFlowCrontab {
	CodeBranch := ""
	if repoType == RepoTypeGIT {
		CodeBranch = branch
	}

	return &EnnFlowCrontab{
		DoCrontabTime: doCrontabTime,
		EnnFlow: EnnFlow{
			FlowId:        ciFlow.FlowId,
			StageId:       "",
			CodeBranch:    CodeBranch,
			LoginUserName: ciFlow.Owner,
			Namespace:     ciFlow.Namespace,
			UserNamespace: ciFlow.Namespace,
		},
	}
}

func (ennCrontab *EnnFlowCrontab) RunCrontab() {

	DoCrontabTime := ennCrontab.DoCrontabTime.Format("04 15 * * * *")

	glog.Infof(DoCrontabTime)
	//crontab.New().MustAddJob("59 23 * * *", RunCrontab) // every minute

	crontab.New().MustAddJob(DoCrontabTime, ennCrontab.Run) // every minute

}

func (ennCrontab *EnnFlowCrontab) Run() {

	//ennFlow.FlowId = flowId
	//ennFlow.StageId = bodyReqBody.StageId
	//ennFlow.CodeBranch = bodyReqBody.Options.Branch
	//ennFlow.LoginUserName = cf.User.Username
	//ennFlow.Namespace = cf.Namespace
	//ennFlow.UserNamespace = cf.User.Namespace

	//不传event参数 event 是代码分支
	imageBuild := models.NewImageBuilder(client.ClusterID)
	stagequeue := NewStageQueueNew(ennCrontab.EnnFlow, "", ennCrontab.EnnFlow.Namespace, ennCrontab.EnnFlow.LoginUserName,
		ennCrontab.EnnFlow.FlowId, imageBuild)

	stagequeue.Namespace = ennCrontab.EnnFlow.Namespace

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
