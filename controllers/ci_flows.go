package controllers

import (
	"dev-flows-api-golang/models"
	"github.com/golang/glog"
	"encoding/json"
	//"gopkg.in/yaml.v2"
	"dev-flows-api-golang/models/common"
	"dev-flows-api-golang/modules/client"
	"dev-flows-api-golang/modules/transaction"
	"fmt"
	"dev-flows-api-golang/util/uuid"
	"time"
	clustermodel "dev-flows-api-golang/models/cluster"
	sqlstatus "dev-flows-api-golang/models/sql/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"dev-flows-api-golang/modules/log"
	"net/http"
	"github.com/ghodss/yaml"
	"strings"
)

var NullTime time.Time

type CiFlowsController struct {
	BaseController
}

//@router / [GET]
func (cf *CiFlowsController) GetCIFlows() {
	method := "CiFlowsController.GetCIFlows"
	ciflows := &models.CiFlows{}
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	isBuildImage, _ := cf.GetInt("isBuildImage", 0)
	listFlowsData, total, err := ciflows.ListFlowsAndLastBuild(namespace, isBuildImage)
	if err != nil || total == 0 {
		glog.Errorf("%s not found the list flow %s\n", method, err)
		cf.ResponseErrorAndCode("No flow added yet", http.StatusOK)
		return
	}

	var flowList []models.ListFlowsInfoResp
	var flowInfo models.ListFlowsInfoResp
	flowList = make([]models.ListFlowsInfoResp, 0)
	var buildInfo models.Build
	for _, flow := range listFlowsData {

		if flow.BuildInfo != "" {
			err = json.Unmarshal([]byte(flow.BuildInfo), &buildInfo)
			if err != nil {
				glog.Errorf("%s json unmarshal failed %s\n", method, err)
				cf.ResponseErrorAndCode("json unmarshal failed", http.StatusBadRequest)
				return
			}
		}
		flowInfo.Image = buildInfo.Image
		if flow.Last_build_id == "" {
			flowInfo.Status = nil
			flowInfo.Last_build_id = nil
			flowInfo.Last_build_time = nil
			flowInfo.Update_time = nil
		} else {
			flowInfo.Status = flow.Status
			flowInfo.Last_build_id = flow.Last_build_id
			flowInfo.Last_build_time = flow.Last_build_time
			flowInfo.Update_time = flow.Update_time
		}
		if flow.Repo_type == "svn" {
			flowInfo.Default_branch = nil
			flowInfo.BuildInfo = nil
		} else {
			flowInfo.Default_branch = flow.Default_branch
			flowInfo.BuildInfo = flow.BuildInfo
		}
		flowInfo.Flow_id = flow.Flow_id
		flowInfo.Name = flow.Name
		flowInfo.Owner = flow.Owner
		flowInfo.Namespace = flow.Namespace
		flowInfo.Init_type = flow.Init_type
		flowInfo.Create_time = flow.Create_time
		flowInfo.Is_build_image = flow.Is_build_image
		flowInfo.Stages_count = flow.Stages_count
		flowInfo.Project_id = flow.Project_id
		flowInfo.Repo_type = flow.Repo_type
		flowInfo.Address = flow.Address

		flowList = append(flowList, flowInfo)

	}

	cf.ResponseSuccessDevops(&flowList, total)
	return
}

//@router / [POST]
func (cf *CiFlowsController) CreateCIFlow() {

	method := "CiFlowsController.CreateCIFlow"

	isBuildImage, _ := cf.GetInt("isBuildImage", 0)
	body := cf.Ctx.Input.RequestBody
	glog.Infof("%s request body:%s\n", body)
	yamlInfo := cf.GetString("o")

	cf.Audit.SetOperationType(models.AuditOperationCreate)
	cf.Audit.SetResourceType(models.AuditResourceFlows)

	contentType := cf.Ctx.Input.Header("content-type")
	if yamlInfo == "yaml" || contentType == "application/yaml" {

		//TODO 暂时不支持yaml创建
		cf.ResponseErrorAndCode("暂时不支持yaml构建", http.StatusConflict)
		return
	} else {
		flows := &models.CiFlows{}
		var flow models.CiFlows
		err := json.Unmarshal(body, &flow)
		if err != nil {
			glog.Errorf("%s create flow json unmarshal failed:%v\n", method, err)
			cf.ResponseErrorAndCode("create flow json unmarshal failed:", http.StatusConflict)
			return
		}
		status, flowid, err := flows.CreateCIFlow(cf.Namespace, cf.User, flow, isBuildImage)
		if err != nil {
			glog.Errorf("%s insert flow info into database:%v\n", method, err)
			cf.ResponseErrorAndCode(err, status)
			return
		}

		cf.ResponseSuccessFLowDevops("Flow created successfully", flowid)
		return
	}

	return
}

type SyncCiFlowRequest struct {
	StageInfo   []models.Stage_info `json:"stage_info"`
	Namespaces  []string `json:"namespaces"`
	EnnFlowName string `json:"flowName"`
}

//@router /:flow_id/sync [POST]
func (cf *CiFlowsController) SyncCIFlow() {

	method := "CiFlowsController.SyncCIFlow"
	flowId := cf.Ctx.Input.Param(":flow_id")
	namespace := cf.Namespace

	ciFlow, err := models.NewCiFlows().FindFlowById(namespace, flowId)
	if err != nil {
		glog.Errorf("not found the flows %s info:%v\n", flowId, err)
		cf.ResponseErrorAndCode("找不到相关的EnnFlow", http.StatusNotFound)
		return
	}

	ciFlow.Owner = cf.User.Username
	ciFlow.CreateTime = time.Now()
	ciFlow.NotificationConfig = ""

	body := cf.Ctx.Input.RequestBody

	glog.Infof("SyncCIFlow the request body is:%v\n", string(cf.Ctx.Input.RequestBody))

	var stageInfos SyncCiFlowRequest

	err = json.Unmarshal(body, &stageInfos)
	if err != nil {
		glog.Errorf("%s sync flow json unmarshal failed:%v\n", method, err)
		cf.ResponseErrorAndCode("sync flow json unmarshal failed:", http.StatusConflict)
		return
	}

	if stageInfos.EnnFlowName == "" {
		cf.ResponseErrorAndCode("flowName is required:", http.StatusBadRequest)
		return
	}

	if len(stageInfos.Namespaces) != 0 {
		ciFlow.Namespace = stageInfos.Namespaces[0]
	} else {
		cf.ResponseErrorAndCode("namespaces is required:", http.StatusBadRequest)
		return
	}

	ciFlow.Name = stageInfos.EnnFlowName

	flowInfo, err := models.NewCiFlows().FindFlowByName(stageInfos.Namespaces[0], ciFlow.Name)
	if err != nil || flowInfo.FlowId != "" {
		cf.ResponseErrorAndCode("同步的目标空间已经存在同名的EnnFlow", http.StatusConflict)
		return
	}

	cf.Audit.SetOperationType(models.AuditOperationCreate)
	cf.Audit.SetResourceType(models.AuditResourceFlows)

	ciFlow.FlowId = uuid.NewCIFlowID()

	var stage models.CiStages
	var script models.CiScripts
	var ciConfig models.CiConfig
	var stageLink models.CiStageLinks
	var dockerfile models.CiDockerfile
	var dockerfiles []models.CiDockerfile
	dockerfiles = make([]models.CiDockerfile, 0)
	var container models.Container
	var buildInfo models.Build

	stageInfoLen := len(stageInfos.StageInfo)

	if stageInfoLen == 0 {
		cf.ResponseErrorAndCode("不能同步子任务为空的EnnFlow", http.StatusBadRequest)
		return
	}

	updateResult := func() bool {

		trans := transaction.New()
		orm := trans.O()
		trans.Do(func() {

			_, err = models.NewCiFlows().CreateOneFlow(ciFlow, orm)
			if err != nil {
				glog.Errorf("%s buildInfo json unmarsh failed:%v\n", method, err)
				trans.Rollback(method, "insert stage info to database failed", err)
				return
			}

			for index, stageInfo := range stageInfos.StageInfo {

				stage.StageId = uuid.NewStageID()

				stage.FlowId = ciFlow.FlowId
				stage.StageName = stageInfo.Metadata.Name
				stage.Seq = index + 1
				stage.ProjectId = ""
				stage.DefaultBranch = stageInfo.Spec.Project.Branch
				stage.Type = stageInfo.Metadata.Type
				stage.CustomType = ""
				stage.Image = stageInfo.Spec.Container.Image

				container.Image = stageInfo.Spec.Container.Image
				container.Args = stageInfo.Spec.Container.Args
				container.Dependencies = stageInfo.Spec.Container.Dependencies
				container.Env = stageInfo.Spec.Container.Env

				stageInfos.StageInfo[index].Metadata.Id = stage.StageId

				if stageInfo.Spec.Container.Scripts_id != "" {
					script.ID = uuid.NewScriptID()
					ciScript := models.NewCiScripts()
					ciScript.GetScriptByID(stageInfo.Spec.Container.Scripts_id)
					script.Content = ciScript.Content
					_, err = ciScript.AddScript(script.ID, script.Content, orm)
					if err != nil {
						glog.Errorf("add script failed:%v\n", err)
						trans.Rollback(method, "insert stage info to database failed", err)
						return
					}

					container.Scripts_id = script.ID

				} else {
					container.Scripts_id = ""
				}

				containerData, err := json.Marshal(container)
				if err != nil {
					glog.Errorf("%s container info json unmarsh failed:%v\n", method, err)
					trans.Rollback(method, "insert stage info to database failed", err)
					return

				}
				stage.ContainerInfo = string(containerData)
				stage.CreationTime = time.Now()

				ciConfig = stageInfo.Spec.Ci.CiConfig
				ciConfigData, err := json.Marshal(ciConfig)
				if err != nil {
					glog.Errorf("%s buildInfo json unmarsh failed:%v\n", method, err)
					trans.Rollback(method, "insert stage info to database failed", err)
					return
				}

				stage.CiConfig = string(ciConfigData)

				stage.CiEnabled = stageInfo.Spec.Ci.Enabled
				//get dockerfile
				if stageInfo.Spec.Build != nil && stageInfo.Metadata.Type == 3 {
					oldDockerfile, err := models.NewCiDockerfile().GetDockerfile(namespace, flowId, stageInfo.Metadata.Id, orm)
					if err != nil {
						parseResult, _ := sqlstatus.ParseErrorCode(err)
						if sqlstatus.SQLErrNoRowFound != parseResult {

							glog.Errorf("%s get dockerfile failed:%v\n", method, err)
							trans.Rollback(method, "insert stage info to database failed", err)
							return
						}

					}

					dockerfile.Content = oldDockerfile.Content
					dockerfile.StageId = stage.StageId
					dockerfile.FlowId = ciFlow.FlowId
					dockerfile.Namespace = stageInfos.Namespaces[0]
					dockerfile.ModifiedBy = ciFlow.Owner
					dockerfile.CreateTime = time.Now().Format("2006-01-02 15:04:05")
					dockerfile.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
					dockerfile.Type = oldDockerfile.Type

					if stageInfo.Spec.Build.DockerfileFrom == 2 {
						dockerfiles = append(dockerfiles, dockerfile)
					}

					buildInfo = *stageInfo.Spec.Build
					buildInfo.Image = ""
					buildInfo.ProjectId = 0
					buildInfo.Project = ""
					buildInfoData, err := json.Marshal(buildInfo)
					if err != nil {
						glog.Errorf("%s buildInfo json unmarsh failed:%v\n", method, err)
						trans.Rollback(method, "insert stage info to database failed", err)
						return
					}
					stage.BuildInfo = string(buildInfoData)
				} else {
					stage.BuildInfo = ""
				}

				_, err = models.NewCiStage().InsertOneStage(stage, orm)
				if err != nil {
					glog.Errorf("%s InsertOneStage to database failed:%v\n", method, err)
					trans.Rollback(method, "insert stage info to database failed", err)
					return
				}

			}

			for index, stageInfo := range stageInfos.StageInfo {

				stageLink.SourceId = stageInfo.Metadata.Id
				if stageInfoLen != index+1 {
					stageLink.TargetId = stageInfos.StageInfo[index+1].Metadata.Id
				} else {
					stageLink.TargetId = ""
				}
				stageLink.FlowId = ciFlow.FlowId
				stageLink.SourceDir = stageInfo.Link.SourceDir
				stageLink.TargetDir = stageInfo.Link.TargetDir
				stageLink.Enabled = int8(stageInfo.Link.Enabled)

				if stageLink.TargetId != "" {
					err = models.NewCiStageLinks().InsertLink(stageLink, orm)
					if err != nil {
						trans.Rollback(method, "insert stage link info to database failed", err)
						glog.Errorf(" add stage link failed:%v\n", err)
						return
					}
				} else {
					err = models.NewCiStageLinks().Insert(stageLink, orm)
					if err != nil {
						trans.Rollback(method, "insert stage link info to database failed", err)
						glog.Errorf(" add stage link failed:%v\n", err)
						return
					}
				}

			}

			if len(dockerfiles) != 0 {
				for _, dfile := range dockerfiles {
					_, err = models.NewCiDockerfile().AddDockerfile(dfile, orm)
					if err != nil {
						glog.Errorf("%s get dockerfile failed:%v\n", method, err)
						trans.Rollback(method, "insert stage info to database failed", err)
						return
					}
				}
			}

		}).Done()

		return trans.IsCommit()
	}

	if !updateResult() {

		cf.ResponseErrorAndCode("Sync flow failed", http.StatusInternalServerError)

		return
	}

	cf.ResponseErrorAndCode("Sync flow success", http.StatusOK)
	return
}

//@router /:flow_id/ci-rules [GET]
func (cf *CiFlowsController) GetCIRules() {
	method := "CiFlowsController.GetCIRules"
	flow_id := cf.Ctx.Input.Param(":flow_id")
	stage := &models.CiStages{}
	stageInfo, err := stage.FindFirstOfFlow(flow_id)

	if err != nil {
		parseNumber, _ := sqlstatus.ParseErrorCode(err)
		if parseNumber == sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s Stage cannot be found %v\n", method, err)
			cf.ResponseErrorAndCode("Stage cannot be found", http.StatusNotFound)
			return
		} else {
			glog.Errorf("%s Get stage info by flowId %s failed from database ERR: %v\n", method, err)
			cf.ResponseErrorAndCode("Lookup Stage failed,please try again!", http.StatusForbidden)
			return
		}
	}
	if stageInfo.StageId == "" {
		glog.Errorf("%s Stage cannot be found %v\n", method, err)
		cf.ResponseErrorAndCode("Stage cannot be found", http.StatusNotFound)
		return
	}

	if stageInfo.CiConfig != "" {
		glog.Infof("%s ci config info %s\n", method, stage.CiConfig)

		var ciConfig models.CiConfig
		err = json.Unmarshal([]byte(stageInfo.CiConfig), &ciConfig)
		if err != nil {
			glog.Errorf("%s ci config json unmarshal failed: %v\n", method, err)
			cf.ResponseErrorAndCode("json 解析失败", http.StatusForbidden)
			return
		}
		cirule := &models.Ci{
			Enabled:  stageInfo.CiEnabled,
			CiConfig: ciConfig,
		}

		cf.ResponseSuccessCIRuleDevops(cirule)
		return
	}

	cf.ResponseErrorAndCode("您还没有开启持续集成", http.StatusNotFound)
	return

}

//{
//  "status": 200,
//  "results": {
//    "enabled": 1,
//    "config": {
//      "branch": {
//        "name": "master",
//        "matchWay": "RegExp"
//      },
//      "tag": {
//        "name": "tag",
//        "matchWay": false
//      },
//      "mergeRequest": false,
//      "buildCluster": "CID-ca4135da3326"
//    }
//  }
//}
//@router /:flow_id/ci-rules [PUT]
func (cf *CiFlowsController) UpdateCIRules() {
	method := "CiFlowsController.UpdateCIRules"
	flow_id := cf.Ctx.Input.Param(":flow_id")

	cirule := cf.Ctx.Input.RequestBody
	glog.Infof("%s request body cirule=[%s]", method, string(cirule))
	if string(cirule) == "" {
		glog.Errorf("%s cf.Ctx.Input.RequestBody is empty \n", method)
		cf.ResponseErrorAndCode("request body is empty", http.StatusBadRequest)
		return
	}

	cf.Audit.SetOperationType(models.AuditOperationUpdate)
	cf.Audit.SetResourceType(models.AuditResourceCIRules)
	var ci models.Ci
	err := json.Unmarshal(cirule, &ci)
	if err != nil {
		glog.Errorf("%s json unmarshal failed err:%v\n", method, err)
		cf.ResponseErrorAndCode("json 解析请求参数失败，请检查转入的参数", http.StatusBadRequest)
		return
	}

	stage := &models.CiStages{}
	stageInfo, err := stage.FindFirstOfFlow(flow_id)
	if err != nil {
		parseNumber, _ := sqlstatus.ParseErrorCode(err)
		if parseNumber == sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s Stage cannot be found %v\n", method, err)
			cf.ResponseErrorAndCode("Stage cannot be found", http.StatusNotFound)
			return
		} else {
			glog.Errorf("%s Get stage info by flowId %s failed from database ERR: %v\n", method, err)
			cf.ResponseErrorAndCode("Lookup Stage failed,please try again!", http.StatusForbidden)
			return
		}
	}
	if stageInfo.StageId == "" {
		glog.Errorf("%s Stage cannot be found %v\n", method, err)
		cf.ResponseErrorAndCode("Stage cannot be found", http.StatusNotFound)
		return
	}

	ciConfigInfo, err := json.Marshal(ci.CiConfig)
	if err != nil {
		glog.Errorf("%s ci config json marshal failed: %v\n", method, err)
		cf.ResponseErrorAndCode("json 系列化失败", http.StatusForbidden)
		return
	}

	stageInfo.CiConfig = string(ciConfigInfo)
	stageInfo.CiEnabled = ci.Enabled
	stage = &stageInfo
	err = stage.UpdateOneById()
	if err != nil {
		glog.Errorf("%s update ci confgi failed:%v\n", method, err)
		cf.ResponseErrorAndCode("update CiRule failed", http.StatusConflict)
		return
	}

	if ci.Enabled == 1 {
		go func() {
			//repoType:SVN GIT
			ciFlow, err := models.NewCiFlows().FindFlowByIdCrab(stageInfo.FlowId)
			if err != nil {
				glog.Errorf("%s failed:%v\n", method, err)
			}
			//open
			if ci.CiConfig.Crontab.Enabled == 1 {

				if EnnCrontab.Exist(flow_id) {
					EnnCrontab.Remove(EnnCrontab.GetCrontabId(flow_id))
				}

				EnnCrontab.RunCrontab(ciFlow, ci.CiConfig.Crontab.CrontabTime, ci.CiConfig.Crontab.RepoType, ci.CiConfig.Crontab.Branch)

				//close
			} else {
				if EnnCrontab.Exist(flow_id) {
					EnnCrontab.Remove(EnnCrontab.GetCrontabId(flow_id))
					EnnCrontab.DeleteIdToMap(flow_id)
					models.NewCiCrontab().EnabledCiCrontab(flow_id, ci.CiConfig.Crontab.CrontabTime,
						0)
				}
			}

		}()
	}

	type Result struct {
		StageId string `json:"stageId"`
	}
	var res Result
	res.StageId = stage.StageId
	cf.ResponseSuccessCIRuleDevops(res)
	return
}

//@router /:flow_id [GET]
func (cf *CiFlowsController) GetCIFlowById() {
	method := "CiFlowsController.GetCIFlowById"
	flow_id := cf.Ctx.Input.Param(":flow_id")
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	//TODO 支持yaml格式
	if cf.Ctx.Input.Header("content-type") == "application/yaml" || cf.GetString("o") == "yaml" {
		ciflow := models.NewCiFlows()
		flowInfo, err := ciflow.FindFlowById(namespace, flow_id)
		if err != nil {
			parseResult, _ := sqlstatus.ParseErrorCode(err)
			if parseResult == sqlstatus.SQLErrNoRowFound {
				glog.Errorf("%s yaml not found the flow %s info from database err: %v \n", method, flow_id, err)
				cf.ResponseErrorAndCode("yaml not found the flow "+flow_id, http.StatusNotFound)
				return
			} else {
				glog.Errorf("%s yaml get the flow %s info from database failed err: %v \n", method, flow_id, err)
				cf.ResponseErrorAndCode("find the flow "+flow_id+"info failed", http.StatusInternalServerError)
				return
			}
		}
		var flowYaml models.FlowYaml
		flowYaml.Kind = "CiFlow"
		flowYaml.Name = flowInfo.Name
		//通知
		var notifica models.NotificationConfig
		if flowInfo.NotificationConfig != "" {
			err := json.Unmarshal([]byte(flowInfo.NotificationConfig), &notifica)
			if err != nil {
				glog.Errorf("%s yaml 文件 json NotificationConfig 解析错误 err:%v\n", method, err)
				cf.ResponseErrorAndCode("yaml 文件  json NotificationConfig 解析错误", http.StatusForbidden)
				return
			}
			flowYaml.Notification = notifica

		}
		//// Get the stages info of the flow
		buildStages, total, err := models.NewCiStage().FindWithLinksByFlowId(flow_id)
		if err != nil {
			glog.Errorf("%s yaml 方式 find links failed %v\n", method, err)
			cf.ResponseErrorAndCode("yaml 方式 find links failed", http.StatusForbidden)
			return
		}

		glog.Infof("%s %d get build status success:%#v", method, total, buildStages)

		StageInfos := make([]models.Stage_info, 0)
		if len(buildStages) != 0 {
			for _, stage := range buildStages {
				StageInfos = append(StageInfos, models.FormatStage(stage))

			}
		}
		stages := make([]models.StageYaml, 0)
		var stage models.StageYaml
		for _, stageInfo := range StageInfos {
			stage.Name = stageInfo.Metadata.Name
			stage.Type = stageInfo.Metadata.Type
			stage.CustomType = stageInfo.Metadata.CustomType
			stage.Project = stageInfo.Spec.Project
			stage.Container = stageInfo.Spec.Container
			stage.Build = stageInfo.Spec.Build
			stage.Ci = stageInfo.Spec.Ci
			stages = append(stages, stage)
		}
		flowYaml.StateYaml = stages
		yamlData, err := json.Marshal(&flowYaml)
		if err != nil {
			glog.Errorf("%s yaml 方式 json marshal failed  %v\n", method, err)
			cf.ResponseErrorAndCode(" yaml 方式 json marshal failed", http.StatusForbidden)
			return
		}
		y, err := yaml.JSONToYAML(yamlData)
		if err != nil {
			glog.Errorf("%s JSONToYAML failed  %v\n", method, err)
			cf.ResponseErrorAndCode("JSONToYAML failed  failed", http.StatusForbidden)
			return
		}

		cf.ResponseResultAndStatusDevops(string(y), http.StatusOK)
		return
		//非yaml格式
	} else {
		ciFLow := models.NewCiFlows()
		ciflowdata, err := ciFLow.FindFlowWithLastBuildById(namespace, flow_id)
		if err != nil {
			parseNumber, _ := sqlstatus.ParseErrorCode(err)
			if parseNumber == sqlstatus.SQLErrNoRowFound {
				glog.Errorf("%s not found the flow info  %v\n", method, err)
				cf.ResponseErrorAndCode("not found the flow info ", http.StatusNotFound)
				return
			} else {
				glog.Errorf("%s get flow info from database failed: err:%v\n", method, err)
				cf.ResponseErrorAndCode("get flow info from database failed\n", http.StatusInternalServerError)
				return
			}

		}

		//// Get the stages info of the flow
		buildStages, total, err := models.NewCiStage().FindWithLinksByFlowId(flow_id)
		if err != nil {
			glog.Errorf("%s FindWithLinksByFlowId total=%d, err:%v\n", method, total, err)
			cf.ResponseErrorAndCode("database FindWithLinksByFlowId  failed", 500)
			return
		}

		//glog.Infof("%s %d get build status success:%#v", method, total, buildStages)
		ciflowdata.Stage_info = make([]models.Stage_info, 0)
		if len(buildStages) != 0 {
			for _, stage := range buildStages {
				ciflowdata.Stage_info = append(ciflowdata.Stage_info, models.FormatStageInfo(stage))

			}
		}
		cf.ResponseSuccessCIRuleDevops(ciflowdata)
		return

	}
	cf.ResponseSuccessCIRuleDevops("no records")
	return
}

//@router /:flow_id [DELETE]
func (cf *CiFlowsController) RemoveCIFlow() {

	method := "CiFlowsController/RemoveCIFlow"
	flow_id := cf.Ctx.Input.Param(":flow_id")
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	cf.Audit.SetResourceID(flow_id)
	cf.Audit.SetOperationType(models.AuditOperationDelete)
	cf.Audit.SetResourceType(models.AuditResourceFlows)

	ciflow := models.NewCiFlows()
	flowinfo, err := ciflow.FindFlowById(namespace, flow_id)
	if err != nil {
		parseNumber, _ := sqlstatus.ParseErrorCode(err)
		if parseNumber == sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s No flow found matching the flow id %v %v \n", method, flowinfo, err)
			cf.ResponseErrorAndCode("No flow found matching the flow id", http.StatusNotFound)
			return
		} else {
			glog.Errorf("%s get flow info from database failed  %v \n", method, err)
			cf.ResponseErrorAndCode("get flow info from database failed", http.StatusInternalServerError)
			return
		}
	}

	glog.Infof("get flow info:%s\n", flowinfo)

	//====================>trans begin
	trans := transaction.New()
	orm := trans.O()
	trans.Do(func() {
		//delete flpw info from database by flowId
		_, err = models.NewCiDockerfile().RemoveByFlowId(namespace, flow_id, orm)
		if err != nil {
			parseNumber, _ := sqlstatus.ParseErrorCode(err)
			if parseNumber == sqlstatus.SQLErrNoRowFound {
				trans.Rollback(method, "insert stage info to database failed", err)
				glog.Errorf("%s No flow found matching the flow id err:%v \n", method, err)
				cf.ResponseErrorAndCode("No flow found matching the flow id", http.StatusNotFound)
				return
			} else {
				trans.Rollback(method, "insert stage info to database failed", err)
				glog.Errorf("%s delete flpw info from database by flowId err: %v \n", method, err)
				cf.ResponseErrorAndCode("delete flpw info from database by flowId failed", http.StatusInternalServerError)
				return
			}

		}
		_, err = ciflow.RemoveFlow(namespace, flow_id, orm)
		if err != nil {
			trans.Rollback(method, "insert stage info to database failed", err)
			glog.Errorf("RemoveFlow failed %s %s \n", method, err)
			cf.ResponseErrorAndCode("delete flowid failed from database", http.StatusInternalServerError)
			return
		}

	}).Done()

	trans.IsCommit()
	//==========>rrans end
	cf.ResponseErrorAndCode("Flow removed successfully", http.StatusOK)
	return
}

//{
//   "init_type": 1,
//  "isBuildImage":0,
//  "name":"Demo_Demo",
//  "notification_config":"notification_config",
//  "yaml":""
// }
//{"email_list":["QINZHAO@ENNEW.CN"],"ci":{"success_notification":true,"failed_notification":true},
// "cd":{"success_notification":true,"failed_notification":true}}
//@router /:flow_id [PUT]
func (cf *CiFlowsController) UpdateCIFlow() {
	method := "CiFlowsController.UpdateCIFlow"
	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	cf.Audit.SetResourceID(flow_id)
	cf.Audit.SetOperationType(models.AuditOperationUpdate)
	cf.Audit.SetResourceType(models.AuditResourceFlows)
	body := cf.Ctx.Input.RequestBody
	glog.Infof("%s request body: %s\n", method, string(body))
	var updateFlowReqBody models.UpdateFlowReqBody
	err := json.Unmarshal(body, &updateFlowReqBody)
	if err != nil {
		glog.Errorf("%s json unmaeshal failed RequestBody  %v \n", method, err)
		cf.ResponseErrorAndCode("json 解析失败", http.StatusForbidden)
		return
	}

	glog.Infof("%s request body NotificationConfigJson %v\n ", method, updateFlowReqBody.Notification_config)

	ciflow := models.NewCiFlows()
	if updateFlowReqBody.Name != "" {
		flowInfo, err := ciflow.FindFlowById(namespace, flow_id)
		if err != nil {
			parseResult, _ := sqlstatus.ParseErrorCode(err)
			if parseResult != sqlstatus.SQLErrNoRowFound {
				glog.Errorf("%s get the flow %s info from database failed err: %v \n", method, flow_id, err)
				cf.ResponseErrorAndCode("find the flow "+flow_id+"info failed", http.StatusConflict)
				return
			}
		}

		if flowInfo.Name != "" && flowInfo.Name == updateFlowReqBody.Name {
			glog.Errorf("%s get the flow %s info from database failed err: %v \n", method, flow_id, err)
			cf.ResponseErrorAndCode("该EnnFlow名字已经存在,请重新输入", http.StatusConflict)
			return
		}

	}

	updateResult, err := ciflow.UpdateFlowById(namespace, flow_id, updateFlowReqBody)
	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult == sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s updateResult=%d  err:%v\n", method, updateResult, err)
			cf.ResponseErrorAndCode("not found the flow "+flow_id, http.StatusNotFound)
			return
		} else {
			glog.Errorf("%s update flow info failed form database: updateResult=%d err:%v\n", method, updateResult, err)
			cf.ResponseErrorAndCode("修改失败 ", http.StatusInternalServerError)
			return
		}
	}

	cf.ResponseSuccessCIRuleDevops("Flow updated successfully")
	return
}

//@router /:flow_id/images [GET]
func (cf *CiFlowsController) GetImagesOfFlow() {

	method := "CiFlowsController.GetImagesOfFlow"

	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	cifolw := models.NewCiFlows()
	flow, err := cifolw.FindFlowById(namespace, flow_id)
	if err != nil || flow.Name == "" {
		glog.Errorf("%s %v\n", method, err)
		cf.ResponseErrorAndCode("No flow found mathcing the flow id", 404)
		return
	}
	stage := models.NewCiStage()
	cistages, total, err := stage.FindBuildEnabledStages(flow_id)
	if err != nil || flow.Name == "" {
		glog.Errorf("FindBuildEnabledStages %s %v total=\n", method, err, total)
		cf.ResponseErrorAndCode("FindBuildEnabledStages from database failed", 500)
		return
	}
	imageList := make([]models.ImageList, 0)
	var image models.ImageList
	var buildInfo models.Build

	if len(cistages) != 0 {
		for _, cistage := range cistages {
			glog.Infof("cistage.BuildInfo=%v", cistage.BuildInfo)
			if cistage.BuildInfo == "" {
				continue
			}
			err := json.Unmarshal([]byte(cistage.BuildInfo), &buildInfo)
			if err != nil {
				glog.Errorf("BuildInfo %s %v total=%d\n", method, err, total)
				cf.ResponseErrorAndCode("json 解析失败", 401)
				return
			}
			if buildInfo.Project != "" {
				image.ImageName = buildInfo.Project + "/" + buildInfo.Image
				image.ProjectId = buildInfo.ProjectId
			} else {
				image.ImageName = common.Default_push_project + "/" + buildInfo.Image
				image.ProjectId = buildInfo.ProjectId
			}

			imageList = append(imageList, image)

		}
	}

	glog.Infof("imageslist = %v\n", imageList)

	cf.ResponseSuccessImageListDevops(imageList)
	return
}

//@router /:flow_id/deployment-logs [GET]
func (cf *CiFlowsController) ListDeploymentLogsOfFlow() {
	method := "CiFlowsController.ListDeploymentLogsOfFlow"
	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	limit, _ := cf.GetInt("limit")
	if limit == 0 {
		limit = 10
	}
	log := models.NewCDDeploymentLogs()
	logs, total, err := log.ListLogsByFlowId(namespace, flow_id, limit)
	if err != nil {
		glog.Errorf("%s total=%d %v\n", method, total, err)
		cf.ResponseErrorAndCode("ListLogsByFlowId failed", 500)
		return
	}
	if total == 0 {
		cf.ResponseSuccessStatusAndMessageDevops("No CD rule created for now")
		return
	}

	type Result struct {
		Status   int `json:"status"`
		Duration string `json:"duration"`
	}

	type ListLog struct {
		App_name         string `json:"app_name"`
		Image_name       string `json:"image_name"`
		Target_version   string `json:"target_version"`
		Cluster_name     string `json:"cluster_name"`
		Upgrade_strategy int `json:"upgrade_strategy"`
		Result           Result  `json:"result"`
		Create_time      string `json:"create_time"`
	}
	listLogs := make([]ListLog, 0)
	listLog := ListLog{}
	var result Result
	for _, log := range logs {
		json.Unmarshal([]byte(log.Result), &result)
		listLog.App_name = log.App_name
		listLog.Target_version = log.Target_version
		listLog.Image_name = log.Image_name
		listLog.Cluster_name = log.Cluster_name
		listLog.Upgrade_strategy = log.Upgrade_strategy
		listLog.Result = result
		listLog.Create_time = log.Create_time.Format("2006-01-02 15:04:05")
		listLogs = append(listLogs, listLog)
	}

	cf.ResponseSuccessDevops(listLogs, total)
	return
}

//@router /:flow_id/cd-rules [GET]
func (cf *CiFlowsController) ListCDRules() {
	method := "CiFlowsController.ListCDRules"
	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	cdRule := models.NewCdRules()
	cdruels, total, err := cdRule.ListRulesByFlowId(namespace, flow_id)
	if err != nil {
		glog.Errorf("%s get rule failed:%v \n", method, err)
		cf.ResponseErrorAndCode("ListRulesByFlowId from database failed ", http.StatusInternalServerError)
		return
	}
	cf.ResponseSuccessDevops(cdruels, total)
	return
}

//@router /:flow_id/cd-rules [POST]
func (cf *CiFlowsController) CreateCDRule() {
	method := "CiFlowsController.CreateCDRule"

	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	cf.Audit.SetOperationType(models.AuditOperationCreate)
	cf.Audit.SetResourceType(models.AuditResourceCDRules)

	body := cf.Ctx.Input.RequestBody
	glog.Infof("%s RequestBody=%v\n", method, string(body))
	var cdRuleReq models.CdRuleReq
	err := json.Unmarshal(body, &cdRuleReq)
	if err != nil {
		glog.Infof("%s json unmarshal failed:%v \n", method, err)
		cf.ResponseErrorAndCode("json 解析失败", http.StatusBadRequest)
		return
	}

	cdRules, total, err := models.NewCdRules().ListRulesByFlowIdAndImageName(cdRuleReq.Image_name, namespace)
	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult != sqlstatus.SQLErrNoRowFound || total != 0 {
			glog.Errorf("%s Binding_service.Deployment_id=%v deployment.ObjectMeta.UID=%s Err:%v\n", cdRules, cdRuleReq.Binding_service.Deployment_id, err)
			var namespaces []string
			namespaces = make([]string, 0)
			for _, cdrule := range cdRules {

				if cdrule.Namespace != namespace {
					namespaces = append(namespaces, cdrule.Namespace)
				}

			}

			cf.ResponseErrorAndCode(fmt.Sprintf("该镜像[%s]的规则已经在空间[%s]存在,不能再添加该镜像[%s]的自动部署规则了",
				cdRuleReq.Image_name, strings.Join(namespaces, ","), cdRuleReq.Image_name), http.StatusBadRequest)
			return
		}
	}

	if total != 0 {
		glog.Warningf("%s Binding_service.Deployment_id=%v deployment.ObjectMeta.UID=%s Err:%v\n", cdRules, cdRuleReq.Binding_service.Deployment_id, err)
		var namespaces []string
		namespaces = make([]string, 0)
		for _, cdrule := range cdRules {

			if cdrule.Namespace != namespace {
				namespaces = append(namespaces, cdrule.Namespace)
			}

		}

		cf.ResponseErrorAndCode(fmt.Sprintf("该镜像[%s]的规则已经在空间[%s]存在,不能再添加该镜像[%s]的自动部署规则了",
			cdRuleReq.Image_name, strings.Join(namespaces, ","), cdRuleReq.Image_name), http.StatusBadRequest)
		return
	}

	//校验rule
	if !models.IsValidRule(cdRuleReq) {
		glog.Errorf("%s Missing required fields of this rule: %v \n", method, cdRuleReq)
		cf.ResponseErrorAndCode("Missing required fields of this rule", http.StatusBadRequest)
		return
	}

	//get kubernetes clientset
	k8sClient := client.GetK8sConnection(cdRuleReq.Binding_service.Cluster_id)
	// Check if the cluster/deployment exists
	if k8sClient == nil {
		glog.Errorf("%s The specified cluster %s not exist\n", method, cdRuleReq.Binding_service.Cluster_id)
		cf.ResponseErrorAndCode("The specified cluster"+cdRuleReq.Binding_service.Cluster_id+" does not exist", http.StatusNotFound)
		return
	}

	deployment, err := k8sClient.ExtensionsV1beta1Client.Deployments(namespace).
		Get(cdRuleReq.Binding_service.Deployment_name, v1.GetOptions{})
	//if err != nil || deployment.Status.Replicas <= 0 {
	if err != nil {
		glog.Errorf("k8sClient get deployment failed or Failed to validate service information %s %v \n", method, err)
		cf.ResponseErrorAndCode("您的服务不存在或者该服务已经停止", http.StatusUnauthorized)
		return
	}
	glog.Infof("deployment.Status.Replicas=%v\n", deployment.Status.Replicas)

	if cdRuleReq.Binding_service.Deployment_id != fmt.Sprintf("%s", deployment.ObjectMeta.UID) {
		glog.Errorf("Binding_service.Deployment_id=%s deployment.ObjectMeta.UID=%s\n", cdRuleReq.Binding_service.Deployment_id, deployment.ObjectMeta.UID)
		cf.ResponseErrorAndCode("The uid of specified service does not match", http.StatusBadRequest)
		return
	}

	glog.Infof("Binding_service.Deployment_id=%s deployment.ObjectMeta.UID=%s\n", cdRuleReq.Binding_service.Deployment_id, deployment.ObjectMeta.UID)

	// Check if the cd rule alreay exists
	cdRule := models.NewCdRules()
	rule, err := cdRule.FindMatchingRule(namespace, flow_id, cdRuleReq.Image_name,
		cdRuleReq.Match_tag, cdRuleReq.Binding_service.Cluster_id, cdRuleReq.Binding_service.Deployment_name)

	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult != sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s Binding_service.Deployment_id=%s deployment.ObjectMeta.UID=%s %v\n", rule, cdRuleReq.Binding_service.Deployment_id, deployment.ObjectMeta.UID, err)
			cf.ResponseErrorAndCode("Service is abnormal, please try again later.", http.StatusInternalServerError)
			return
		}
	}

	//if exist int the database will return
	if rule.Enabled == 1 || rule.MatchTag != "" {
		glog.Infof("%s Binding_service.Deployment_id=%s deployment.ObjectMeta.UID=%s\n", rule, cdRuleReq.Binding_service.Deployment_id, deployment.ObjectMeta.UID)
		cf.ResponseErrorAndCode("CD rule matching the same conditions already exists", http.StatusConflict)
		return
	}

	var newRuleInfo models.CDRules
	newRuleInfo.RuleId = uuid.NewCDRuleID()
	newRuleInfo.CreateTime = time.Now()
	newRuleInfo.UpdateTime = time.Now()
	newRuleInfo.BindingClusterId = cdRuleReq.Binding_service.Cluster_id
	newRuleInfo.BindingDeploymentId = cdRuleReq.Binding_service.Deployment_id
	newRuleInfo.BindingDeploymentName = cdRuleReq.Binding_service.Deployment_name
	newRuleInfo.Namespace = namespace
	newRuleInfo.FlowId = flow_id
	newRuleInfo.ImageName = cdRuleReq.Image_name
	newRuleInfo.UpgradeStrategy = cdRuleReq.Upgrade_strategy
	newRuleInfo.MatchTag = cdRuleReq.Match_tag
	newRuleInfo.Enabled = 1
	newRuleInfo.MinReadySeconds = cdRuleReq.MinReadySeconds

	glog.Infof("%s newRuleInfo.RuleId=%s \n", method, newRuleInfo.RuleId)
	res, err := cdRule.CreateOneRule(newRuleInfo)
	if err != nil {
		glog.Errorf("%s create cd rule failed res:%d err:%v", method, res, err)
		cf.ResponseErrorAndCode(" create cd rule failed ", http.StatusInternalServerError)
		return
	}

	var cdRuleResp models.CdRuleResp
	cdRuleResp.Rule_id = newRuleInfo.RuleId
	cdRuleResp.Message = "CD rule was created successfully"

	cf.ResponseCreateSuccessCDRuleDevops(cdRuleResp.Message, cdRuleResp.Rule_id)
	return
}

//@router /:flow_id/cd-rules/:rule_id [DELETE]
func (cf *CiFlowsController) RemoveCDRule() {

	method := "CiFlowsController.RemoveCDRule"

	flowId := cf.Ctx.Input.Param(":flow_id")

	ruleId := cf.Ctx.Input.Param(":rule_id")
	cf.Audit.SetResourceID(ruleId)
	cf.Audit.SetOperationType(models.AuditOperationDelete)
	cf.Audit.SetResourceType(models.AuditResourceCDRules)
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	rule := models.NewCdRules()
	result, err := rule.RemoveRule(namespace, flowId, ruleId)
	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult == sqlstatus.SQLErrNoRowFound {
			glog.Warningf("%s not found the cd rule: %d %v\n", method, err)
			cf.ResponseErrorAndCode("No rule found mathcing the rule id", http.StatusNotFound)
			return
		} else {
			glog.Errorf("%s delete cd rule failed:flowId=%s,ruleId=%s, err:%v\n", method, flowId, ruleId, err)
			cf.ResponseErrorAndCode("No rule found mathcing the rule id", http.StatusInternalServerError)
			return
		}
	}

	glog.Infof("delete success result:%d number", result)
	cf.ResponseErrorAndCode("CD rule was removed successfully", http.StatusOK)
	return
}

//@router /:flow_id/cd-rules/:rule_id [PUT]
func (cf *CiFlowsController) UpdateCDRule() {
	method := "CiFlowsController.UpdateCDRule"
	flowId := cf.Ctx.Input.Param(":flow_id")
	ruleId := cf.Ctx.Input.Param(":rule_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	cf.Audit.SetResourceID(ruleId)
	cf.Audit.SetOperationType(models.AuditOperationUpdate)
	cf.Audit.SetResourceType(models.AuditResourceCDRules)

	var cdRuleReq models.CdRuleReq
	cdRule := models.CDRules{}

	err := json.Unmarshal(cf.Ctx.Input.RequestBody, &cdRuleReq)
	if err != nil {
		glog.Errorf("%s json Unmarshal failed:%v\n", method, err)
		cf.ResponseErrorAndCode("request body is illicit", http.StatusBadRequest)
		return
	}

	cdRule.UpdateTime = time.Now()
	cdRule.BindingClusterId = cdRuleReq.Binding_service.Cluster_id
	cdRule.BindingDeploymentId = cdRuleReq.Binding_service.Deployment_id
	cdRule.BindingDeploymentName = cdRuleReq.Binding_service.Deployment_name
	cdRule.RuleId = ruleId
	cdRule.ImageName = cdRuleReq.Image_name
	cdRule.FlowId = cdRuleReq.FlowId
	cdRule.MatchTag = cdRuleReq.Match_tag
	cdRule.UpgradeStrategy = cdRuleReq.Upgrade_strategy
	cdRule.MinReadySeconds = cdRuleReq.MinReadySeconds

	updateResult, err := models.NewCdRules().UpdateCDRule(namespace, flowId, ruleId, cdRule)
	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult == sqlstatus.SQLErrNoRowFound {
			glog.Errorf("%s %v\n", method, err)
			cf.ResponseErrorAndCode("No rule found mathcing the rule id", 404)
			return
		} else {
			glog.Errorf("%s %v\n", method, err)
			cf.ResponseErrorAndCode("No rule found mathcing the rule id", http.StatusForbidden)
			return
		}
	}

	if updateResult < 1 {
		glog.Warningf("%s update failed updateResult:%d\n", method, updateResult)
		cf.ResponseErrorAndCode("CD rule was updated failed", http.StatusForbidden)
		return
	}

	cf.ResponseErrorAndCode("CD rule was updated successfully", http.StatusOK)
	return
}

//@router /:flow_id/builds [GET]
func (cf *CiFlowsController) ListBuilds() {
	method := "CiFlowsController.ListBuilds"
	flow_id := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	limit, _ := cf.GetInt("limit")
	if limit == 0 {
		limit = common.DEFAULT_PAGE_SIZE
	}

	log := models.NewCiFlowBuildLogs()
	logs, total, err := log.FindAllOfFlow(flow_id, limit)
	if err != nil {
		glog.Errorf("%s find build log from database failed :total=%d %v\n", method, total, err)
		cf.ResponseErrorAndCode("ListBuilds failed", http.StatusInternalServerError)
		return
	}
	flowBuildResp := make([]models.FlowBuildLogResp, 0)

	for _, flowBuild := range logs {
		flowBuildResp = append(flowBuildResp, models.FormatBuild(flowBuild, ""))
	}

	resp := struct {
		Results []models.FlowBuildLogResp `json:"results"`
	}{
		Results: flowBuildResp,
	}

	cf.ResponseSuccessStatusAndResultDevops(resp)
	return
}

//@router /:flow_id/builds [POST]
func (cf *CiFlowsController) CreateFlowBuild() {

	method := "controllers/CiFlowsController/CreateFlowBuild"

	flowId := cf.Ctx.Input.Param(":flow_id")
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	body := cf.Ctx.Input.RequestBody
	if string(body) == "" {
		glog.Warningf("%s %s\n", method, "RequestBody is empty")
		cf.ResponseErrorAndCode("RequestBody is empty", http.StatusConflict)
		return
	}
	//审计
	cf.Audit.SetOperationType(models.AuditOperationStart)
	cf.Audit.SetResourceType(models.AuditResourceBuilds)

	var bodyReqBody models.BuildReqbody

	err := json.Unmarshal(body, &bodyReqBody)
	if err != nil {
		glog.Errorf("%s json unmarshal bodyReqBody failed: %v\n", method, err)
		cf.ResponseErrorAndCode("body json 解析失败", http.StatusUnauthorized)
		return
	}

	var ennFlow EnnFlow
	ennFlow.FlowId = flowId
	ennFlow.StageId = bodyReqBody.StageId
	ennFlow.CodeBranch = bodyReqBody.Options.Branch
	ennFlow.LoginUserName = cf.User.Username
	ennFlow.Namespace = cf.Namespace
	ennFlow.UserNamespace = cf.User.Namespace

	if ok := FlowMapping.Exist(flowId); !ok {
		cf.ResponseErrorAndCode("网络异常，请刷新页面", http.StatusContinue)
		return
	}

	//不传event参数 event 是代码分支
	imageBuild := models.NewImageBuilder(client.ClusterID)
	stagequeue := NewStageQueueNew(ennFlow, "", ennFlow.Namespace, ennFlow.LoginUserName, flowId, imageBuild)

	if stagequeue != nil {
		//判断是否该EnnFlow当前有执行中
		err := stagequeue.CheckIfBuiding(flowId)
		if err != nil {
			glog.Warningf("%s Too many waiting builds of:  %v\n", method, err)
			if strings.Contains(fmt.Sprintf("%s", err), "该EnnFlow已有任务在执行,请等待执行完再试") {
				cf.ResponseErrorAndCode("该EnnFlow ["+stagequeue.CiFlow.Name+"] 已有任务在执行,请等待执行完再试", http.StatusConflict)
				return
			} else {
				cf.ResponseErrorAndCode("找不到对应的EnnFlow", http.StatusNotFound)
				return
			}
		}
		//开始执行 把执行日志插入到数据库
		stagequeue.InsertLog()
		var Resp interface{}
		Resp = struct {
			FlowBuildId  string `json:"flowBuildId"`
			StageBuildId string `json:"stageBuildId"`
		}{
			FlowBuildId:  stagequeue.StageBuildLog.FlowBuildId,
			StageBuildId: stagequeue.StageBuildLog.BuildId,
		}

		cf.ResponseResultAndStatusDevops(Resp, http.StatusOK)

		go stagequeue.Run()
		return
	}

	cf.ResponseErrorAndCode("服务异常,请稍后再识试", http.StatusInternalServerError)
	return

}

type StageBuilds struct {
	BuildId      string `json:"buildId"`
	CreationTime string `json:"creationTime"`
	EndTime      string`json:"endTime"`
	StartTime    string `json:"startTime"`
	Status       int `json:"status"`
	StageName    string `json:"stageName"`
	StageId      string `json:"stageId"`
}

type BuildResult struct {
	BuildId      string `json:"buildId"`
	FlowId       string `json:"flowId"`
	CreationTime time.Time `json:"creationTime"`
	EndTime      time.Time `json:"endTime"`
	StartTime    time.Time `json:"startTime"`
	Status       int `json:"status"`
	StageBuilds  []StageBuilds `json:"stageBuilds"`
}

//@router /:flow_id/lastbuild [GET]
func (cf *CiFlowsController) GetLastBuildDetails() {

	method := "CiFlowsController/GetLastBuildDetails"

	flowId := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	flowBuild := models.NewCiFlowBuildLogs()

	builds, total, err := flowBuild.FindLastBuildOfFlowWithStages(flowId)
	if err != nil {
		glog.Errorf("%s Get flow info failed from database: %v\n", method, err)
		cf.ResponseErrorAndCode("not found the flows info", http.StatusNotFound)
		return
	}

	if total != 0 {
		var buildResult BuildResult
		var stageBuilds StageBuilds
		buildResult.StageBuilds = make([]StageBuilds, 0)
		buildResult.BuildId = builds[0].BuildId
		buildResult.FlowId = builds[0].FlowId
		buildResult.CreationTime = builds[0].CreationTime
		buildResult.EndTime = builds[0].EndTime
		buildResult.StartTime = builds[0].StartTime
		buildResult.Status = builds[0].Status

		for _, build := range builds {
			stageBuilds.BuildId = build.StageBuildBuildId
			stageBuilds.CreationTime = build.StageBuildCreationTime
			stageBuilds.StartTime = build.StageBuildStartTime
			stageBuilds.StageName = build.StageName
			stageBuilds.EndTime = build.StageBuildEndTime
			stageBuilds.Status = build.StageBuildStatus
			stageBuilds.StageId = build.StageId
			buildResult.StageBuilds = append(buildResult.StageBuilds, stageBuilds)
		}

		results := struct {
			Results BuildResult `json:"results"`
		}{
			Results: buildResult,
		}

		cf.ResponseResultAndStatusDevops(results, http.StatusOK)
		return
	}

	cf.ResponseErrorAndCode("没有相关的EnnFlow的构建记录", http.StatusNotFound)
	return
}

//@router /:flow_id/builds/:flow_build_id [GET]
func (cf *CiFlowsController) ListStagesBuilds() {

	method := "CiFlowsController.ListStagesBuilds"

	flowId := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}

	flowBuildId := cf.Ctx.Input.Param(":flow_build_id")

	flowBuild := models.NewCiFlowBuildLogs()

	build, err := flowBuild.FindFlowBuild(flowId, flowBuildId)
	if err != nil {
		glog.Errorf("%s get flowBuild info failed: build:%v, err:%v\n", method, build, err)
		cf.ResponseErrorAndCode("Cannot find flow build of specified flow", http.StatusNotFound)
		return
	}

	stageBuilds, total, err := models.NewCiStageBuildLogs().FindAllOfFlowBuild(flowBuildId)
	if err != nil {
		glog.Errorf("%s get stage buildLog failed from database:%v \n", method, err)
		cf.ResponseErrorAndCode("Cannot find flow build of specified flow", http.StatusNotFound)
		return
	}

	results := struct {
		Results []models.CiStageBuildLogs `json:"results"`
	}{
		Results: stageBuilds,
	}

	if total != 0 {
		cf.ResponseResultAndStatusDevops(results, http.StatusOK)
		return
	}

	cf.ResponseErrorAndCode("Cannot find flow build of specified flow", http.StatusNotFound)
	return
}

//@router /:flow_id/stages/:stage_id/builds/:build_id/stop [PUT]
func (cf *CiFlowsController) StopBuild() {

	method := "CiFlowsController.StopBuild"
	//更新flow构建状态
	flowId := cf.Ctx.Input.Param(":flow_id")
	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	stageId := cf.Ctx.Input.Param(":stage_id")

	buildId := cf.Ctx.Input.Param(":build_id")

	cf.Audit.SetOperationType(models.AuditOperationStop)
	cf.Audit.SetResourceType(models.AuditResourceBuilds)

	stageInfo, err := models.NewCiStage().FindOneById(stageId)
	if err != nil {
		glog.Errorf("%s find stage failed from database:%v\n", method, err)
		cf.ResponseErrorAndCode("not found this stage", http.StatusNotFound)
		return
	}

	if stageInfo.FlowId != flowId {
		cf.ResponseErrorAndCode("Stage "+stageId+" does not belong to Flow "+flowId, http.StatusForbidden)
		return
	}

	stageBuilds, err := models.NewCiStageBuildLogs().FindStageBuild(stageId, buildId)
	if err != nil || stageBuilds.FlowBuildId == "" {
		glog.Errorf("%s find stage build failed: %v \n", method, err)
		cf.ResponseErrorAndCode("Stage build not found", http.StatusNotFound)
		return
	}

	//已构建完成，直接返回
	if stageBuilds.Status < common.STATUS_BUILDING {
		cf.ResponseErrorAndCode("build is not running", http.StatusOK)
		return
	}

	flowBuildId := stageBuilds.FlowBuildId
	glog.Infof("%s %s", method, " Stopping flow "+flowBuildId)
	//获取未完成的stage构建
	builds, total, err := models.NewCiStageBuildLogs().FindUnfinishedByFlowBuildId(flowBuildId)
	if err != nil {
		glog.Errorf("%s find stage build log failed: %v \n", method, err)
		cf.ResponseErrorAndCode("build is not running", http.StatusOK)
		return
	}
	if total == 0 {
		glog.Infof("%s build is not running \n", method)
		var ennFlow EnnFlow
		ennFlow.FlowId = flowId
		ennFlow.Status = http.StatusOK
		ennFlow.Flag = 1
		ennFlow.FlowBuildId = flowBuildId
		ennFlow.StageId = stageId
		ennFlow.StageBuildId = buildId
		ennFlow.BuildStatus = common.STATUS_FAILED
		ennFlow.Message = "停止失败"
		FlowMapping.Send(ennFlow)
		_, err = models.NewCiFlowBuildLogs().UpdateById(time.Now(), common.STATUS_FAILED, flowBuildId)
		if err != nil {
			glog.Errorf("%s update flowBuild log failed: %v \n", method, err)
		}
		cf.NewSuccessStatusFlowBuildIdDevops("build is not running", flowBuildId)
		return
	}

	//遍历未完成构建，删除对应job并更新数据库
	//绝大多数情况只会有一条未完成构建
	imageBuilder := models.NewImageBuilder()

	for _, build := range builds {
		buildRec := models.CiStageBuildLogs{
			EndTime: time.Now(),
			Status:  common.STATUS_FAILED,
		}
		if build.Status == common.STATUS_BUILDING {
			if build.Namespace != "" && build.JobName != "" {
				if build.PodName == "" {
					podName, err := imageBuilder.GetPodName(build.Namespace, build.JobName)
					if err != nil {
						glog.Errorf("%s get podName failed: %v \n", method, err)
					}
					if podName != "" {
						buildRec.PodName = podName
					}
				}

				job, err := imageBuilder.StopJob(build.Namespace, build.JobName, true, 1)
				if err != nil {
					glog.Errorf("%s Failed to delete job of stage build: build=%s,job.message=%v, err:%v \n", method, build.BuildId, job.Status, err)
				}

				pod, err := imageBuilder.GetPod(job.ObjectMeta.Namespace,
					job.ObjectMeta.Name, build.BuildId)
				if err != nil {
					glog.Errorf("%s get pod info of %s from kubernetes failed:%v\n", method, job.ObjectMeta.Name, err)
					//stageBuildResp.Message = "get pod failed from kubernetes"
				}

				models.NewCiStageBuildLogs().UpdateStageBuildNodeNameAndPodNameById(pod.Spec.NodeName, pod.ObjectMeta.Name, build.BuildId)

			}
		}

		//更新stage构建状态
		var ennFlow EnnFlow
		ennFlow.FlowId = flowId
		ennFlow.Status = http.StatusOK
		ennFlow.Flag = 1
		ennFlow.FlowBuildId = flowBuildId
		ennFlow.StageId = stageId
		ennFlow.StageBuildId = buildId
		ennFlow.BuildStatus = common.STATUS_FAILED
		ennFlow.Message = "停止任务成功"
		FlowMapping.Send(ennFlow)
		_, err = models.NewCiStageBuildLogs().UpdateBuildLogById(buildRec, build.BuildId)
		if err != nil {
			glog.Errorf("%s update stage build status failed : err:%v \n", method, err)
		}

	}

	_, err = models.NewCiFlowBuildLogs().UpdateById(time.Now(), common.STATUS_FAILED, flowBuildId)
	if err != nil {
		glog.Errorf("%s update flowBuild log failed: %v \n", method, err)
	}
	cf.NewSuccessStatusFlowBuildIdDevops("Success", flowBuildId)
	return
}

//@router /:flow_id/stages/:stage_id/builds [GET]
func (cf *CiFlowsController) ListBuildsOfStage() {
	method := "CiFlowsController.ListBuildsOfStage"
	flowId := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	stageId := cf.Ctx.Input.Param(":stage_id")

	stageInfo, err := models.NewCiStage().FindOneById(stageId)
	if err != nil {
		glog.Errorf("%s Stage cannot be found: %v\n", method, err)
		cf.ResponseErrorAndCode("Stage cannot be found", http.StatusNotFound)
		return
	}

	if stageInfo.FlowId != flowId {
		cf.ResponseErrorAndCode("Stage "+stageId+" does not belong to Flow "+flowId, http.StatusConflict)
		return
	}

	stageBuilds, total, err := models.NewCiStageBuildLogs().FindAllOfStage(stageId, common.DEFAULT_PAGE_SIZE)
	if err != nil {
		glog.Errorf("%s get stage build log failed: total=%d, err:%v \n", method, total, err)
		cf.ResponseErrorAndCode("Stage build log not found", http.StatusNotFound)
		return
	}

	results := struct {
		Results []models.CiStageBuildLogs `json:"results"`
	}{
		Results: stageBuilds,
	}

	cf.ResponseResultAndStatusDevops(results, http.StatusOK)
	return
}

//@router /:flow_id/stages/:stage_id/builds/:stage_build_id/log [GET]
func (cf *CiFlowsController) GetStageBuildLogsFromES() {

	method := "CiFlowsController/GetStageBuildLogsFromES"

	flowId := cf.Ctx.Input.Param(":flow_id")

	stageId := cf.Ctx.Input.Param(":stage_id")
	stageBuildId := cf.Ctx.Input.Param(":stage_build_id")

	imageBuilder := models.NewImageBuilder()
	//valid the flowdata and stagedata
	build, err := GetValidStageBuild(flowId, stageId, stageBuildId)
	if err != nil {
		glog.Errorf("%s get log from ES failed:%v\n", method, err)
		cf.Ctx.ResponseWriter.Write([]byte(`<font color="red">[Enn Flow API Error] 找不到相关日志，请稍后重试!</font>`))
		return
	}

	if build.TimeOut == 1 {
		cf.Ctx.ResponseWriter.Write([]byte(`<font color="red">[Enn Flow API Error] 构建任务启动超时或者启动失败，没有相关的日志!</font>`))
		cf.Ctx.ResponseWriter.Status = 200
		return
	}

	namespace := build.Namespace

	if build.PodName == "" {
		podName, err := imageBuilder.GetPodName(build.Namespace, build.JobName, build.BuildId)
		if err != nil || podName == "" {
			glog.Errorf("%s get job name=[%s] pod name failed from kubernetes:%v\n", method, build.JobName, err)
		}
		models.NewCiStageBuildLogs().UpdatePodNameById(podName, build.BuildId)
		build.PodName = podName
	}

	startTime := build.CreationTime
	if build.StartTime != NullTime {
		startTime = build.StartTime
	}

	endTime := build.EndTime

	cf.Ctx.ResponseWriter.Write([]byte(`-----------------------------------------------------------------------------<br/>`))
	if build.NodeName != "" {
		cf.Ctx.ResponseWriter.Write([]byte(`构建节点名称:` + build.NodeName + `, 子任务容器: 仅显示最近  200  条日志 <br/>`))
	} else {
		cf.Ctx.ResponseWriter.Write([]byte(`子任务容器: 仅显示最近  200  条日志 <br/>`))
	}
	//cf.Ctx.ResponseWriter.Write([]byte(`搜集日志需要时间，请耐心等待... <br/>`))
	cf.Ctx.ResponseWriter.Write([]byte(`-----------------------------------------------------------------------------<br/>`))

	var indexs []string

	days1 := fmt.Sprintf("logstash-%s", startTime.Add(-8 * time.Hour).Format("2006.01.02"))
	days2 := fmt.Sprintf("logstash-%s", endTime.Add(-8 * time.Hour).Format("2006.01.02"))

	if days1 == days2 {
		indexs = append(indexs, days1)
	} else {
		indexs = append(indexs, days1)
		indexs = append(indexs, days2)
	}

	//当从es获取日志失败的时候，从k8s获取日志
	// get cluster info
	cluster := &clustermodel.ClusterModel{}
	errno, err := cluster.Get(client.ClusterID)
	if err != nil {
		if errno == sqlstatus.SQLErrNoRowFound {
			glog.Errorln(method, "cluster", client.ClusterID, "not found")
			cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] 找不到构建集群的信息</font>`))
			return
		}
		glog.Errorln(method, "get cluster info from database failed", err)
		cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] 找不到构建集群的信息</font>`))
		return
	}
	// get logs from kubernetes client function
	getLogFromK8S := func() error {
		glog.Infof("will get log from kubernetes.......\n")

		err = imageBuilder.ESgetLogFromK8S(namespace, build.PodName, models.SCM_CONTAINER_NAME, cf.Ctx)
		if err != nil {
			return err
		}

		err = imageBuilder.ESgetLogFromK8S(namespace, build.PodName, models.BUILDER_CONTAINER_NAME, cf.Ctx)
		if err != nil {
			return err
		}
		return nil
	}

	//如果创建失败
	if build.Status != 0 {
		eventlist, err := imageBuilder.GetPodEvents(namespace, build.PodName, "type!=Normal")
		if err != nil {
			glog.Errorf("%s get pod events failed: %v\n", method, err)
		}
		if len(eventlist.Items) != 0 {
			for _, event := range eventlist.Items {
				cf.Ctx.ResponseWriter.Write([]byte(fmt.Sprintf(`<font color="#ffc20e">[%s][%s]</font> %s <br/>`, event.CreationTimestamp.Format("2006/01/02 15:04:05"), event.Type, event.Message)))

			}

		}
	}

	if time.Now().Sub(endTime) < 20*time.Second {
		err = getLogFromK8S()
		if err != nil {
			//get log client
			logClient, err := log.NewESClient("")
			if logClient == nil || err != nil {
				glog.Errorf("NewESClient failed: %v\n", err)
				glog.Infoln("will get log form kubernetes \n")
				getLogFromK8S()
				cf.Ctx.ResponseWriter.Status = 200
				return
			}

			var containerNames = []string{models.SCM_CONTAINER_NAME, models.BUILDER_CONTAINER_NAME}
			err = logClient.SearchTodayLog(indexs, namespace, containerNames, build.PodName, client.ClusterID, cf.Ctx)
			if err != nil {
				glog.Errorf("get logs from es failed,will get logs form kubernetes err:%v\n", err)
				getLogFromK8S()
				cf.Ctx.ResponseWriter.Status = 200
				cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] PAAS平台只保留7天之内的日志信息 </font>`))
				return
			}
		}

	} else if time.Now().Sub(endTime) <= 7*24*time.Hour || 20*time.Second < time.Now().Sub(endTime) {
		//get log client
		logClient, err := log.NewESClient("")
		if logClient == nil || err != nil {
			glog.Errorf("NewESClient failed: %v\n", err)
			glog.Infoln("will get log form kubernetes \n")
			getLogFromK8S()
			cf.Ctx.ResponseWriter.Status = 200
			return
		}

		var containerNames = []string{models.SCM_CONTAINER_NAME, models.BUILDER_CONTAINER_NAME}
		err = logClient.SearchTodayLog(indexs, namespace, containerNames, build.PodName, client.ClusterID, cf.Ctx)
		if err != nil {
			glog.Errorf("get logs from es failed,will get logs form kubernetes err:%v\n", err)
			getLogFromK8S()
			cf.Ctx.ResponseWriter.Status = 200
			cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] PAAS平台只保留7天之内的日志信息 </font>`))
			return
		}

		glog.Infof("will get log form ES successfully ")

	} else {
		cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] 没有更早的日志了 </font>`))

	}

	cf.Ctx.ResponseWriter.Status = 200
	cf.Ctx.ResponseWriter.Write([]byte(`<font color="#ffc20e">[Enn Flow API] PAAS平台只保留7天之内的日志信息 </font>`))
	return

}

//@router /:flow_id/stages/:stage_id/builds/:stage_build_id/events [GET]
func (cf *CiFlowsController) GetBuildEvents() {
	method := "CiFlowsController.GetBuildEvents"
	flowId := cf.Ctx.Input.Param(":flow_id")

	namespace := cf.Namespace
	if namespace == "" {
		namespace = cf.Ctx.Input.Header("usernmae")
	}
	stageId := cf.Ctx.Input.Param(":stage_id")
	stageBuildId := cf.Ctx.Input.Param(":stage_build_id")

	imageBuilder := models.NewImageBuilder()
	//valid the flowdata and stagedata
	build, err := GetValidStageBuild(flowId, stageId, stageBuildId)
	if err != nil {
		glog.Errorf("%s get log from ES failed:%v\n", method, err)
		cf.ResponseErrorAndCode("stageBuild info not found", http.StatusNotFound)
		return
	}

	if build.PodName == "" {
		podName, err := imageBuilder.GetPodName(build.Namespace, build.JobName)
		if err != nil || podName == "" {
			glog.Errorf("%s get job name=[%s] pod name failed from kubernetes:%v\n", method, build.JobName, err)
			cf.ResponseErrorAndCode(`<font color="#ffc20e">[Enn Flow API] 构建任务不存在或已经被删除</font>`, http.StatusNotFound)
			return
		}
		models.NewCiStageBuildLogs().UpdatePodNameById(podName, build.BuildId)
		build.PodName = podName
	}

	eventListJob, err := imageBuilder.GetJobEvents(namespace, build.JobName, build.PodName)
	if err != nil {
		glog.Errorf("%s get job event failed:%v\n", method, err)
		return
	}
	eventListPod, err := imageBuilder.GetPodEvents(namespace, build.PodName, "")
	if err != nil {
		glog.Errorf("%s get job event failed:%v\n", method, err)
		return
	}
	copy(eventListJob.Items, eventListPod.Items)
	cf.ResponseSuccessDevops(eventListJob.Items, int64(len(eventListJob.Items)))
	return
}

// getDays helper function to generate date list from start to end
func getDays(start, end string) ([]time.Time, error) {
	var days []time.Time
	var startDate, endDate time.Time
	var err error
	// defaultDate := "2000-01-01"
	if start != "" {
		//start = start + "T00:00:00Z"
		startDate, err = time.Parse(time.RFC3339, start)
		if err != nil {
			return days, fmt.Errorf("failed to parse start date %s", err.Error())
		}
	}
	startDate = startDate.Add(-8 * time.Hour)
	days = append(days, startDate)
	if end != "" {
		//end = end + "T23:59:59Z"
		endDate, err = time.Parse(time.RFC3339, end)
		if err != nil {
			return days, fmt.Errorf("failed to parse end date %s", err.Error())
		}
	}
	endDate = endDate.Add(-8 * time.Hour)

	days = append(days, endDate)
	fmt.Printf("days:%v\n", days)
	return days, nil
}
