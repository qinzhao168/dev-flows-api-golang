package models

import (
	"time"
	"github.com/astaxie/beego/orm"
	"github.com/golang/glog"
	"dev-flows-api-golang/models/user"
	"dev-flows-api-golang/util/uuid"
	"errors"
	"dev-flows-api-golang/models/common"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type CiFlows struct {
	FlowId    string `orm:"pk;column(flow_id)" json:"flow_id"`
	Name      string `orm:"column(name)" json:"name"`
	Owner     string `orm:"column(owner)" json:"owner"`
	Namespace string `orm:"column(namespace)" json:"namespace"`
	//新建flow的方式
	InitType           int8 `orm:"column(init_type)" json:"init_type"`
	NotificationConfig string `orm:"column(notification_config)" json:"notification_config"`
	CreateTime         time.Time `orm:"column(create_time)" json:"create_time"`
	UpdateTime         time.Time `orm:"column(update_time)" json:"update_time"`
	UniformRepo        int8 `orm:"column(uniform_repo)" json:"uniform_repo"`
	IsBuildImage       int8 `orm:"column(is_build_image)" json:"isBuildImage"`
	Yaml               string `orm:"-" json:"yaml"`
}

type BuildReqbody struct {
	StageId string `json:"stageId"`
	Options *Option `json:"options"`
}

type Option struct {
	Branch string `json:"branch"`
}
type CiFlowsCount struct {
	FlowId    string `orm:"pk;column(flow_id)" json:"flow_id"`
	Name      string `orm:"column(name)" json:"name"`
	Owner     string `orm:"column(owner)" json:"owner"`
	Namespace string `orm:"column(namespace)" json:"namespace"`
	//新建flow的方式
	InitType           int8 `orm:"column(init_type)" json:"init_type"`
	NotificationConfig string `orm:"column(notification_config)" json:"notification_config"`
	CreateTime         time.Time `orm:"column(create_time)" json:"create_time"`
	UpdateTime         time.Time `orm:"column(update_time)" json:"update_time"`
	UniformRepo        int8 `orm:"column(uniform_repo)" json:"uniform_repo"`
	IsBuildImage       int8 `orm:"column(is_build_image)" json:"isBuildImage"`
	Num                int `orm:"column(num)" json:"yaml"`
}

type ListFlowsInfo struct {
	Flow_id         string `orm:"column(flow_id)" json:"flow_id"`
	Name            string `orm:"column(name)" json:"name"`
	Owner           string `orm:"column(owner)" json:"owner"`
	Namespace       string `orm:"column(namespace)" json:"namespace"`
	Init_type       int `orm:"column(init_type)" json:"init_type"`
	Create_time     time.Time `orm:"column(create_time)" json:"create_time"`
	Update_time     time.Time `orm:"column(update_time)" json:"update_time,omitempty"`
	Last_build_time string `orm:"column(last_build_time)" json:"last_build_time,omitempty"`
	Status          int `orm:"column(status)" json:"status"`
	Last_build_id   string `orm:"column(last_build_id)" json:"last_build_id,omitempty"`
	Is_build_image  int `orm:"column(is_build_image)" json:"is_build_image"`
	Stages_count    int `orm:"column(stages_count)" json:"stages_count"`
	Project_id      string `orm:"column(project_id)" json:"project_id"`
	Repo_type       string `orm:"column(repo_type)" json:"repo_type"`
	Default_branch  string `orm:"column(default_branch)" json:"default_branch"`
	Address         string `orm:"column(address)" json:"address"`
	BuildInfo       string `orm:"column(buildInfo)" json:"buildInfo"`
	Image           string `orm:"-" json:"image"`
}

type ListFlowsInfoResp struct {
	Flow_id         string `orm:"column(flow_id)" json:"flow_id"`
	Name            string `orm:"column(name)" json:"name"`
	Owner           string `orm:"column(owner)" json:"owner"`
	Namespace       string `orm:"column(namespace)" json:"namespace"`
	Init_type       int `orm:"column(init_type)" json:"init_type"`
	Create_time     time.Time `orm:"column(create_time)" json:"create_time"`
	Update_time     interface{} `orm:"column(update_time)" json:"update_time"`
	Last_build_time interface{} `orm:"column(last_build_time)" json:"last_build_time"`
	Status          interface{} `orm:"column(status)" json:"status"`
	Last_build_id   interface{} `orm:"column(last_build_id)" json:"last_build_id"`
	Is_build_image  int `orm:"column(is_build_image)" json:"is_build_image"`
	Stages_count    int `orm:"column(stages_count)" json:"stages_count"`
	Project_id      string `orm:"column(project_id)" json:"project_id"`
	Repo_type       string `orm:"column(repo_type)" json:"repo_type"`
	Default_branch  interface{} `orm:"column(default_branch)" json:"default_branch"`
	Address         string `orm:"column(address)" json:"address"`
	BuildInfo       interface{} `orm:"column(buildInfo)" json:"buildInfo"`
	Image           string `orm:"-" json:"image"`
}

type CiFlowsResp struct {
	FlowId             string `orm:"pk;column(flow_id)" json:"flow_id"`
	Name               string `orm:"column(name)" json:"name"`
	Owner              string `orm:"column(owner)" json:"owner"`
	Namespace          string `orm:"column(namespace)" json:"namespace"`
	InitType           int8 `orm:"column(init_type)" json:"init_type"`
	NotificationConfig string `orm:"column(notification_config)" json:"notification_config,omitempty"`
	//NotificationConfigResp NotificationConfig `orm:"-" json:"notification_config,omitempty"`
	CreateTime    time.Time `orm:"column(create_time)" json:"create_time"`
	UpdateTime    time.Time `orm:"column(update_time)" json:"update_time"`
	UniformRepo   int8 `orm:"column(uniform_repo)" json:"uniform_repo"`
	IsBuildImage  int8 `orm:"column(is_build_image)" json:"is_build_image"`
	LastBuildTime string `orm:"column(last_build_time)" json:"last_build_time"`
	Status        int `orm:"column(status)" json:"status"`
	Stage_info    []Stage_info `orm:"-" json:"stage_info"`
}

type NotificationConfig struct {
	Email_list []string `json:"email_list,omitempty"`
	Ci         NotiConfigCI `json:"ci,omitempty"`
	Cd         NotiConfigCD `json:"cd,omitempty"`
}

type NotiConfigCI struct {
	Success_notification bool `json:"success_notification,omitempty"`
	Failed_notification  bool `json:"failed_notification,omitempty"`
}

type NotiConfigCD struct {
	Success_notification bool `json:"success_notification,omitempty"`
	Failed_notification  bool `json:"failed_notification,omitempty"`
}

type BuildInfo struct {
	ClusterID       string
	BUILD_INFO_TYPE int
	RepoUrl         string
	IsCodeRepo      int
	Branch          string
	PublicKey       string
	PrivateKey      string
	Git_repo_url    string
	RepoType        string
	ScmImage        string
	Clone_location  string
	Namespace       string
	Build_image     string
	BuildImageFlag  bool
	FlowName        string
	StageName       string
	FlowBuildId     string
	StageBuildId    string
	Type            int
	TargetImage     Build
	ImageOwner      string
	NodeName        string
	Command         []string
	Build_command   []string
	Env             []apiv1.EnvVar
	Dependencies    []Dependencie
	Svn_username    string
	Svn_password    string
}

type BuildRec struct {
	Start_time time.Time
}

func (cf *CiFlows) TableName() string {
	return "tenx_ci_flows"
}

func NewCiFlows() *CiFlows {
	return &CiFlows{}
}

// List flow for specified user
func (cf *CiFlows) ListFlowsAndLastBuild(namespace string, isBuildImage int, orms ...orm.Ormer) (listFlowsInfo []ListFlowsInfo, total int64, err error) {
	sql := "select tmp_flow.*, scount.stages_count, project_id, repo_type, default_branch, address, build_info as buildInfo " +
	//tmp_flow查找flow信息和最后一次构建时间和状态
		"from (select f.flow_id, f.name, f.owner, f.namespace, f.init_type, f.create_time, f.update_time, b.start_time as last_build_time, b.status, b.build_id as last_build_id, f.is_build_image " +
		"from tenx_ci_flows f left join " +
	//l1查找flow的最近构建时间，再根据时间查出构建记录
		"(select l2.* " +
		"from (select flow_id, max(start_time) as last_build_time " +
		"from tenx_ci_flow_build_logs " +
		"where flow_id in (select flow_id from tenx_ci_flows where namespace = ?) " +
		"group by flow_id) l1 left join " +
		"tenx_ci_flow_build_logs l2 on l1.flow_id = l2.flow_id and l1.last_build_time = l2.start_time) b " +
		"on f.flow_id = b.flow_id where namespace = ? ) tmp_flow left join " +
	//scount查找flow对应的stage数量
		"(select f.flow_id, count(s.stage_id) as stages_count, project_id, repo_type, s.build_info, default_branch, p.address, f.is_build_image " +
		"from tenx_ci_flows f left join " +
		"tenx_ci_stages s on s.flow_id = f.flow_id left join tenx_ci_managed_projects p on s.project_id = p.id " +
		"where f.namespace= ? group by f.flow_id) scount " +
		"on scount.flow_id = tmp_flow.flow_id  where tmp_flow.is_build_image = ? " +
		" group by tmp_flow.flow_id order by tmp_flow.create_time desc"

	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	total, err = o.Raw(sql, namespace, namespace, namespace, isBuildImage).QueryRows(&listFlowsInfo)
	if err != nil {
		glog.Errorf("ListFlowsAndLastBuild get data from database failed:%v\n", err)
	}
	return

}

// Add a flow
func (cf *CiFlows) CreateCIFlow(namespace string, user *user.UserModel, body CiFlows, isBuildImage int, orms ...orm.Ormer) (int, string, error) {

	status, message, ciflows := cf.checkAndGenFlow(namespace, user, body)
	if status > 209 {
		return status, ciflows.FlowId, errors.New(message)

	}

	result, err := cf.CreateOneFlow(ciflows)
	if err != nil {
		glog.Errorf("code=%d Error=%s", result, err)
		return status, ciflows.FlowId, err
	}

	return status, ciflows.FlowId, nil

}

func (cf *CiFlows) CreateOneFlow(flow CiFlows, orms ...orm.Ormer) (int64, error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	return o.Insert(&flow)
}

//init_type : 1
//isBuildImage : 0
//name : "ddmo"
//notification_config :
//"{"email_list":["QINZHAO@ENNEW.CN"],"ci":{"success_notification":true,"failed_notification":true},"cd":{"success_notification":true,"failed_notification":true}}"
//yaml : null
func (cf *CiFlows) checkAndGenFlow(namespace string, user *user.UserModel, flows CiFlows, orms ...orm.Ormer) (int, string, CiFlows) {
	var status int
	message := ""

	if user.Username == "" || flows.Name == "" {
		status = 400
		message = "Missing flow name"
		return status, message, flows
	}

	if flows.InitType == 0 || (flows.InitType != 1 && flows.InitType != 2) {
		status = 400
		message = "Invalid init_type, must be 1 (user interface) or 2 (yaml)"
		return status, message, flows
	}

	flow, err := cf.FindFlowByName(namespace, flows.Name)
	if err == nil && flow.FlowId != "" {
		glog.Warningf("the flow already exist: %s", flows.Name)
		status = 409
		message = "Flow (name - '" + flows.Name + "') already exists"
		return status, message, flows
	}

	if err != nil {
		glog.Errorf("get flow info by name failed from database: %s err:%v\n", flows.Name, err)
		status = 500
		message = "服务错误,请稍后再试"
		return status, message, flows
	}

	flows.FlowId = uuid.NewCIFlowID()
	flows.Owner = user.Username
	flows.Namespace = namespace
	flows.CreateTime = time.Now()

	status = 200
	message = "check ok"
	return status, message, flows

}

func (cf *CiFlows) FindFlowByName(namespace, flowName string, orms ...orm.Ormer) (CiFlows, error) {
	method := "models/CiFlows.FindFlowByName"
	var o orm.Ormer
	ciflow := CiFlows{Name: flowName, Namespace: namespace}
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	err := o.Read(&ciflow, "name", "namespace")
	if err == orm.ErrNoRows {
		glog.Infof("%s flow info:%v\n", method, ciflow)
		return ciflow, nil
	}

	return ciflow, err

}

func (cf *CiFlows) FindFlowWithLastBuildById(namespace, flowId string, orms ...orm.Ormer) (ciflows CiFlowsResp, err error) {
	method := "CiFlows.FindFlowWithLastBuildById"
	SELECT_FLOW_WITH_LAST_BUILD_BY_ID := "select f.*, l.start_time as last_build_time, l.status as status " +
		"from tenx_ci_flows f left join tenx_ci_flow_build_logs l on f.flow_id = l.flow_id " +
		"where f.flow_id = ? and f.namespace = ? order by last_build_time desc limit 1"
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	glog.Infof("%s FindFlowWithLastBuildById from database Success\n", method)
	err = o.Raw(SELECT_FLOW_WITH_LAST_BUILD_BY_ID, flowId, namespace).QueryRow(&ciflows)
	return
}

func (cf *CiFlows) FindFlowById(namespace, flowId string, orms ...orm.Ormer) (ciflow CiFlows, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
		Filter("namespace", namespace).One(&ciflow)
	return
}

func (cf *CiFlows) FindFlowByIdCrab(flowId string, orms ...orm.Ormer) (ciflow CiFlows, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
		One(&ciflow)
	return
}

func (cf *CiFlows) RemoveFlow(namespace, flowId string, orms ...orm.Ormer) (deleteRes int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	deleteRes, err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
		Filter("namespace", namespace).Delete()
	if err != nil {
		glog.Errorf("RemoveFlow flowId =%s failed err=[%v] \n", flowId, err)
	}
	return
}

func (cf *CiFlows) UpdateFlowById(namespace, flowId string, flow UpdateFlowReqBody, orms ...orm.Ormer) (updateRes int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	//update flow name
	if flow.Name != "" {
		updateRes, err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
			Filter("namespace", namespace).Update(orm.Params{
			"name":                flow.Name,
			"notification_config": flow.Notification_config,
			"update_time":         time.Now(),
		})

		return
	}

	updateRes, err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
		Filter("namespace", namespace).Update(orm.Params{
		"notification_config": flow.Notification_config,
		"update_time":         time.Now(),
	})

	if err != nil {
		glog.Errorf("UpdateFlowById flowId =%s failed err=[%v] \n", flowId, err)
	}
	return
}

func (cf *CiFlows) Update(namespace, flowId string, uniformRepo int8, orms ...orm.Ormer) (updateRes int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	updateRes, err = o.QueryTable(cf.TableName()).Filter("flow_id", flowId).
		Filter("Namespace", namespace).Update(orm.Params{
		"uniform_repo": uniformRepo,
		"update_time":  time.Now(),
	})
	if err != nil {
		glog.Errorf("Update flowId =%s of uniform_repo failed err=[%v] \n", flowId, err)
	}
	return
}

func (cf *CiFlows) CountBySpace(namespace string, orms ...orm.Ormer) (count int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	count, err = o.QueryTable(cf.TableName()).
		Filter("Namespace", namespace).Count()
	if err != nil {
		glog.Errorf("CountBySpace count =%d failed err=[%v] \n", count, err)
	}
	return
}

func (cf *CiFlows) FindWithDockerfileCountById(namespace, flow_id string, orms ...orm.Ormer) (ciFlowsCount CiFlowsCount, count int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	SELECT_FLOWS_WITH_DOCKERFILE_COUNT := "SELECT f.*, count(stage_id) AS num FROM tenx_ci_flows AS f LEFT JOIN tenx_ci_dockerfiles AS d ON f.flow_id = d.flow_id " +
		"WHERE f.flow_id = ? AND f.namespace = ?"
	count, err = o.Raw(SELECT_FLOWS_WITH_DOCKERFILE_COUNT, flow_id, namespace).QueryRows(&ciFlowsCount)
	if err != nil {
		glog.Errorf("FindWithDockerfileCountById flow_id =%d failed err=[%v] \n", flow_id, err)
	}
	return
}

//flow 状态。0-成功 1-失败 2-执行中  stage状态。0-成功 1-失败 2-执行中 3-等待
func InsertBuildLog(flowBuildRec *CiFlowBuildLogs, stageBuild *CiStageBuildLogs, stageId string, orms ...orm.Ormer) error {
	method := "models/ci_flows/InsertBuildLog"
	o := orm.NewOrm()
	err := o.Begin()
	if err != nil {
		return err
	}
	flowBuildRec.StartTime = flowBuildRec.CreationTime
	sqlFlowBuild := `INSERT INTO tenx_ci_flow_build_logs(build_id, flow_id, creation_time, start_time, end_time, status,branch,creater) VALUES (?, ?, ?, ?, ?, ?,?,?);`
	_, err = o.Raw(sqlFlowBuild, flowBuildRec.BuildId, flowBuildRec.FlowId, flowBuildRec.CreationTime, flowBuildRec.StartTime, flowBuildRec.EndTime, common.STATUS_BUILDING,
		flowBuildRec.Branch, flowBuildRec.Creater).Exec()
	if err != nil {
		glog.Errorf("%s flowBuildRec %v \n", method, err)
		o.Rollback()
		return err
	}
	if !o.QueryTable(NewCiStageBuildLogs().TableName()).
		Filter("stage_id", stageId).Filter("status", common.STATUS_BUILDING).Exist() {
		//没有执行中的构建记录，则添加“执行中”状态的构建记录
		stageBuild.Status = common.STATUS_BUILDING
	}

	sqlstageBuild := `INSERT INTO tenx_ci_stage_build_logs(build_id, flow_build_id, stage_id,
	stage_name, status, job_name,pod_name,node_name,namespace,creation_time,start_time,
	 end_time,build_alone,is_first,branch_name) VALUES (?, ?, ?, ?, ?, ?,?,?,?,?,?,?,?,?,?);`
	_, err = o.Raw(sqlstageBuild, stageBuild.BuildId, stageBuild.FlowBuildId,
		stageBuild.StageId, stageBuild.StageName, stageBuild.Status, stageBuild.JobName,
		stageBuild.PodName, stageBuild.NodeName, stageBuild.Namespace, stageBuild.CreationTime,
		stageBuild.StartTime, stageBuild.EndTime, stageBuild.BuildAlone,
		stageBuild.IsFirst, stageBuild.BranchName).Exec()
	if err != nil {
		glog.Errorf("%s stageBuild %v \n", method, err)
		o.Rollback()
		return err
	}

	err = o.Commit()
	if err != nil {
		return err
	}
	return nil

}
