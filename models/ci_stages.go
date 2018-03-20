package models

import (
	"time"
	"github.com/astaxie/beego/orm"
	"fmt"
	"dev-flows-api-golang/models/user"
	"github.com/golang/glog"
	"errors"
	"encoding/json"
	"dev-flows-api-golang/models/common"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type CiStages struct {
	StageId       string `orm:"pk;column(stage_id)"`
	FlowId        string `orm:"column(flow_id)"`
	StageName     string `orm:"column(stage_name)"`
	Seq           int `orm:"column(seq)"`
	ProjectId     string `orm:"column(project_id)"`
	DefaultBranch string `orm:"column(default_branch)"`
	Type          int `orm:"column(type)"`
	CustomType    string  `orm:"column(custom_type)"` //自定义类型文本值
	Image         string `orm:"column(image)"`
	ContainerInfo string `orm:"column(container_info)"`
	BuildInfo     string `orm:"column(build_info)"`
	CiEnabled     int8 `orm:"column(ci_enabled)"`
	CiConfig      string `orm:"column(ci_config)"`
	CreationTime  time.Time `orm:"column(creation_time)"`
	Option        *Option `orm:"-"`
}

type UpdateFlowReqBody struct {
	Init_type              int `json:"init_type"`
	IsBuildImage           int `json:"isBuildImage"`
	Name                   string `json:"name"`
	Notification_config    string `json:"notification_config"`
	NotificationConfigJson NotificationConfig `json:"-"`
	Yaml                   string `json:"yaml"`
}

type CiStagesMaxSeq struct {
	StageId       string `orm:"pk;column(stage_id)"`
	FlowId        string `orm:"column(flow_id)"`
	StageName     string `orm:"column(stage_name)"`
	Seq           int `orm:"column(seq)"`
	ProjectId     string `orm:"column(project_id)"`
	DefaultBranch string `orm:"column(default_branch)"`
	Type          int `orm:"column(type)"`
	CustomType    string  `orm:"column(custom_type)"` //自定义类型文本值
	Image         string `orm:"column(image)"`
	ContainerInfo string `orm:"column(container_info)"`
	BuildInfo     string `orm:"column(build_info)"`
	CiEnabled     int8 `orm:"column(ci_enabled)"`
	CiConfig      string `orm:"column(ci_config)"`
	CreationTime  time.Time `orm:"column(creation_time)"`
	Maxseq        time.Time `orm:"column(maxseq)"`
}

type Stage_info struct {
	LastBuildStatus LastBuildStatus `json:"lastBuildStatus,omitempty"`
	Link            Link `json:"link,omitempty"`
	Metadata        Metadata `json:"metadata,omitempty"`
	Spec            Spec `json:"spec,omitempty"`
}

type StagesOfFlow struct {
	Flow_id             string `json:"flow_id"`
	Name                string `json:"name"`
	Owner               string `json:"owner"`
	Namespace           string `json:"namespace"`
	Init_type           int `json:"init_type"`
	Notification_config NotificationConfig `json:"notification_config"`
}

type Metadata struct {
	Name         string `json:"name"`
	Id           string `json:"id"`
	CreationTime time.Time `json:"creationTime"`
	Type         int `json:"type"`
	CustomType   string `json:"customType,omitempty"`
}

type Spec struct {
	Container   Container `json:"container,omitempty"`
	Ci          Ci `json:"ci,omitempty"`
	Project     Project `json:"project,omitempty"`
	Build       *Build `json:"build,omitempty"`
	UniformRepo int8 `json:"uniformRepo,omitempty"`
}

type Ci struct {
	Enabled  int8 `json:"enabled,omitempty"`
	CiConfig CiConfig  `json:"config,omitempty"`
}

type CiConfig struct {
	Crontab      Crontab `json:"crontab,omitempty"`
	Branch       Branch `json:"branch,omitempty"`
	Tag          Tag `json:"tag,omitempty"`
	MergeRequest bool `json:"mergeRequest,omitempty"`
	BuildCluster string `json:"buildCluster,omitempty"`
}

type Crontab struct {
	RepoType    string `json:"repoType,omitempty"`
	Enabled     int8 `json:"enabled,omitempty"`
	CrontabTime time.Time `json:"crontabTime"`
	Branch      string `json:"branch,omitempty"`
}

type Branch struct {
	Name     string `json:"name,omitempty"`
	MatchWay interface{} `json:"matchWay,omitempty"`
}

type Tag struct {
	Name     string `json:"name,omitempty"`
	MatchWay interface{} `json:"matchWay,omitempty"`
}

type Build struct {
	DockerfileFrom int `json:"DockerfileFrom,omitempty"`
	DockerfileName string `json:"DockerfileName,omitempty"`
	DockerfilePath string `json:"DockerfilePath,omitempty"`
	CustomTag      string `json:"customTag,omitempty"`
	Image          string `json:"image,omitempty"`
	ImageTagType   int `json:"imageTagType,omitempty"`
	NoCache        bool `json:"noCache"`
	Project        string `json:"project,omitempty"`
	ProjectId      int `json:"projectId,omitempty"`
	RegistryType   int `json:"registryType,omitempty"`
	DockerfileOL   string `json:"DockerfileOL,omitempty,omitempty"`
	CustomRegistry string `json:"customRegistry,omitempty,omitempty"`
}

type StageYaml struct {
	Name       string `json:"name"`
	Type       int `json:"type"`
	CustomType string `json:"custom_type,omitempty"`
	Project    Project `json:"project,omitempty"`
	Container  Container `json:"container,omitempty"`
	Build      *Build `json:"build,omitempty"`
	Ci         Ci `json:"ci,omitempty"`
	Link       Link `json:"link,omitempty"`
}

type FlowYaml struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Notification NotificationConfig `json:"notification"`
	StateYaml    []StageYaml `json:"stages"`
}

type Container struct {
	Scripts_id   string `json:"scripts_id,omitempty"`
	Image        string `json:"image,omitempty"`
	Args         []string `json:"args"`              //使用命令
	Dependencies []Dependencie `json:"dependencies"` //依赖
	Env          []apiv1.EnvVar `json:"env"`
	Command      []string `json:"command,omitempty"`
}

type Dependencie struct {
	Service string `json:"service,omitempty"`
	Env     []apiv1.EnvVar `json:"env,omitempty"`
}

type Env struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Project struct {
	Id       string `json:"id,omitempty"`
	Branch   string `json:"branch,omitempty"`
	RepoType string `json:"repo_type,omitempty"`
}

type Link struct {
	Target    string `json:"target,omitempty"`
	Enabled   int `json:"enabled"`
	SourceDir string `json:"sourceDir,omitempty"`
	TargetDir string `json:"targetDir,omitempty"`
}

type LastBuildStatus struct {
	BuildId  string `json:"buildId"`
	Status   int `json:"status"`
	Pod_name string `json:"pod_name"`
}

type Stages struct {
	Build_id       string `orm:"column(build_id)"`
	Status         int `orm:"column(status)"`
	Pod_name       string `orm:"column(pod_name)" json:"pod_name"`
	Stage_id       string `orm:"column(stage_id)" json:"stage_id"`
	Flow_id        string `orm:"column(flow_id)" json:"flow_id"`
	Stage_name     string `orm:"column(stage_name)" json:"stage_name"`
	Seq            int `orm:"column(seq)" json:"seq"`
	Project_id     string `orm:"column(project_id)" json:"project_id"`
	Default_branch string `orm:"column(default_branch)" json:"default_branch"`
	Type           int `orm:"column(type)" json:"type"`
	Custom_type    string `orm:"column(custom_type)" json:"custom_type"`
	Image          string `orm:"column(image)" json:"image"`
	Container_info string `orm:"column(container_info)" json:"container_info"`
	Build_info     string `orm:"column(build_info)" json:"build_info"`
	Ci_enabled     int8 `orm:"column(ci_enabled)" json:"ci_enabled"`
	Ci_config      string `orm:"column(ci_config)" json:"ci_config"`
	Creation_time  time.Time `orm:"column(creation_time)" json:"creation_time"`
	Source_id      string `orm:"column(source_id)" json:"source_id"`
	Target_id      string `orm:"column(target_id)" json:"target_id"`
	Source_dir     string `orm:"column(source_dir)" json:"source_dir"`
	Target_dir     string `orm:"column(target_dir)" json:"target_dir"`
	Enabled        int `orm:"column(enabled)" json:"enabled"`
	Link_enabled   int `orm:"column(link_enabled)" json:"link_enabled"`
	Repo_type      string `orm:"column(repo_type)" json:"repo_type"`
}

func (cs *CiStages) TableName() string {

	return "tenx_ci_stages"

}

func NewCiStage() *CiStages {
	return &CiStages{}
}

func (cs *CiStages) FindByProjectId(projectId string) error {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where project_id=?", cs.TableName())
	return o.Raw(sql, projectId).QueryRow(&cs)
}

func (cs *CiStages) FindFirstOfFlow(flowId string) (stage CiStages, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where flow_id=? order by seq limit 1", cs.TableName())

	err = o.Raw(sql, flowId).QueryRow(&stage)

	return
}

func (cs *CiStages) FindAllFlowByFlowId(flowId string) (stages []CiStages, total int64, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where flow_id=? order by seq ", cs.TableName())

	total, err = o.Raw(sql, flowId).QueryRows(&stages)

	return
}

func (cs *CiStages) InsertOneStage(stage CiStages, orms ...orm.Ormer) (int64, error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	return o.Insert(&stage)

}

func (cs *CiStages) CountBySpace(namespace string) (int, error) {
	var stageCount int
	o := orm.NewOrm()
	SELECT_STAGES_COUNT_BY_NAMESPACE := "SELECT COUNT(*) AS num FROM tenx_ci_flows AS f " +
		"JOIN tenx_ci_stages AS s ON f.flow_id = s.flow_id WHERE namespace = ?"
	err := o.Raw(SELECT_STAGES_COUNT_BY_NAMESPACE, namespace).QueryRow(&stageCount)
	return stageCount, err

}

func (cs *CiStages) UpdateOneById(orms ...orm.Ormer) error {
	method := "CiStages.UpdateOneById"
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	total, err := o.QueryTable(cs.TableName()).
		Filter("stage_id", cs.StageId).Filter("flow_id", cs.FlowId).Update(orm.Params{
		"ci_enabled": cs.CiEnabled,
		"ci_config":  cs.CiConfig,
	})
	if err != nil {
		glog.Errorf("%s update dockerfile failed:%v\n", method, err)
		return err
	}
	glog.Infof("update stage by stageId result roesAffected:%d", total)
	return nil

}

func (cs *CiStages) UpdateById(stageId string, ciStage CiStages, orms ...orm.Ormer) error {
	method := "models.CiStages.UpdateById"
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	total, err := o.QueryTable(cs.TableName()).
		Filter("stage_id", stageId).Update(orm.Params{
		"stage_name":     ciStage.StageName,
		"build_info":     ciStage.BuildInfo,
		"project_id":     ciStage.ProjectId,
		"custom_type":    ciStage.CustomType,
		"container_info": ciStage.ContainerInfo,
		"image":          ciStage.Image,
		"default_branch": ciStage.DefaultBranch,
		"type":           ciStage.Type,
		"ci_enabled":     ciStage.CiEnabled,
		"ci_config":      ciStage.CiConfig,
	})
	if err != nil {
		glog.Errorf("%s update dockerfile failed:%v\n", method, err)
		return err
	}
	glog.Infof("update stage by stageId result roesAffected:%d", total)
	return nil

}

func (cs *CiStages) Update(flowId, projectId, defaultBranch string, orms ...orm.Ormer) error {
	method := "CiStages.Update"
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	total, err := o.QueryTable(cs.TableName()).
		Filter("flow_id", flowId).Update(orm.Params{
		"project_id":     projectId,
		"default_branch": defaultBranch,
	})
	if err != nil {
		glog.Errorf("%s update dockerfile failed:%v\n", method, err)
		return err
	}
	glog.Infof("update stage by stageId result roesAffected:%d", total)
	return nil

}

func (cs *CiStages) DeleteById(stageId string, orms ...orm.Ormer) (int64, error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	return o.QueryTable(cs.TableName()).Filter("stage_id", stageId).Delete()

}

func (cs *CiStages) FindOneById(stageId string) (stage CiStages, err error) {
	o := orm.NewOrm()
	err = o.QueryTable(cs.TableName()).Filter("stage_id", stageId).One(&stage)
	return
}

func (cs *CiStages) FindByIds(flowId, ids string) (stages []CiStages, total int64, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where flow_id='%s' and stage_id in (%s)", cs.TableName(), flowId, ids)
	total, err = o.Raw(sql).QueryRows(&stages)
	return
}

func (cs *CiStages) FindAllStage() (stages []CiStages, total int64, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where seq=1 ", cs.TableName())
	total, err = o.Raw(sql).QueryRows(&stages)
	return
}

func (cs *CiStages) FindOneByName(flowId, name string) (stages CiStages, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where flow_id = ? and stage_name = ? ", cs.TableName())
	err = o.Raw(sql, flowId, name).QueryRow(&stages)
	return
}

func (cs *CiStages) FindFlowMaxSeq(flowId string) (seq int, err error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select max(seq) from %s where flow_id = ?", cs.TableName())
	err = o.Raw(sql, flowId).QueryRow(&seq)
	return
}

func (cs *CiStages) FindNextOfFlow(flowId string, seq int) (stages CiStages, err error) {
	o := orm.NewOrm()
	err = o.QueryTable(cs.TableName()).Filter("flow_id", flowId).Filter("seq__gt", seq).
		OrderBy("seq").One(&stages)
	return
}

func (cs *CiStages) FindWithLinksByFlowId(flowId string) (stages []Stages, total int64, err error) {
	o := orm.NewOrm()
	sql := "select tenx_ci_stage_build_logs.build_id, tenx_ci_stage_build_logs.status, tenx_ci_stage_build_logs.pod_name, tenx_ci_stages.*, link.*, link.enabled as link_enabled, repo_type " +
		"from " +
	//根据flow_id查询所有stage的最后一次构建时间
		"(select stage.stage_id, max(build.creation_time) as build_time, repo_type " +
		"from tenx_ci_stages as stage left join tenx_ci_stage_build_logs as build on stage.stage_id = build.stage_id " +
		"LEFT JOIN tenx_ci_managed_projects AS project ON stage.project_id = project.id " +
		"where stage.flow_id = ? " +
		"group by stage.stage_id) as last_build_status " +
	//使用最后一次构建时间查询最后一次构建记录
		"left join tenx_ci_stage_build_logs " +
		"on last_build_status.stage_id = tenx_ci_stage_build_logs.stage_id " +
		"and last_build_status.build_time = tenx_ci_stage_build_logs.creation_time " +
	//查询stage信息
		"join tenx_ci_stages on last_build_status.stage_id = tenx_ci_stages.stage_id " +
	//查询link信息
		"join tenx_ci_stage_links as link on last_build_status.stage_id = source_id " +
		"order by seq"
	total, err = o.Raw(sql, flowId).QueryRows(&stages)
	return
}

func (cs *CiStages) FindExpectedLast(flowId, stageId string) (stages CiStages, err error) {
	o := orm.NewOrm()
	sql := "select * from tenx_ci_stages, (select max(seq) as maxseq from tenx_ci_stages where flow_id = ?) as m " +
		"where stage_id = ? and seq = m.maxseq"
	err = o.Raw(sql, flowId, stageId).QueryRow(&stages)
	return
}

func UpdateStageOfFlow(user *user.UserModel, flowId, stageId string, stage CiStages) {
	method := "UpdateStageOfFlow"
	glog.Infof("%s \n", method)

}

func CheckAndSetDefaults(stage CiStages, user *user.UserModel) {
	method := "CheckAndSetDefaults"
	glog.Infof("%s \n", method)

}

func checkRequired(stage CiStages) {
	//if stage
}

//根据stageId获取stage，并检查flow与stage的从属关系是否正确
func (cs *CiStages) GetAndCheckMemberShip(flowId, stageId string) (CiStages, error) {
	method := "CiStages.GetAndCheckMemberShip"
	stage, err := cs.FindOneById(stageId)
	if err != nil {
		glog.Infof("%s %s", method, err)
		return stage, err
	}

	if stage.FlowId == flowId {

		glog.Info("%s %s", "Stage belong to Flow")

		return stage, nil
	} else if stage.FlowId != flowId {

		glog.Errorf("%s %s", method, "Stage does not belong to Flow")

		return stage, errors.New("Stage does not belong to Flow")
	}

	return stage, fmt.Errorf("%s", "Stage cannot be found")
}

func FormatStage(stage Stages) (stage_info Stage_info) {
	method := "models/FormatStage"
	stage_info.Metadata.Name = stage.Stage_name
	stage_info.Metadata.Id = stage.Stage_id
	stage_info.Metadata.CreationTime = stage.Creation_time
	stage_info.Metadata.Type = stage.Type
	if common.CUSTOM_STAGE_TYPE == stage.Type {
		stage_info.Metadata.CustomType = stage.Custom_type
	}
	stage_info.Spec.Ci.Enabled = stage.Ci_enabled
	if stage.Project_id != "" {
		stage_info.Spec.Project.Id = stage.Project_id
		stage_info.Spec.Project.Branch = stage.Default_branch
		stage_info.Spec.Project.RepoType = stage.Repo_type
	}
	if stage.Ci_config != "" {
		err := json.Unmarshal([]byte(stage.Ci_config), &stage_info.Spec.Ci.CiConfig)
		if err != nil {
			glog.Errorf("%s json unmarshal ci config failed: %v\n", method, err)
			return
		}
	}

	if stage.Container_info != "" {
		err := json.Unmarshal([]byte(stage.Container_info), &stage_info.Spec.Container)
		if err != nil {
			glog.Errorf("%s json unmarshal ci Container failed: %v\n", method, err)
			return
		}
	}

	stage_info.Spec.Container.Image = stage.Image
	if stage.Build_info != "" {
		err := json.Unmarshal([]byte(stage.Build_info), &stage_info.Spec.Build)
		if err != nil {
			glog.Errorf("%s json unmarshal ci Build_info failed:%v\n", method, err)
			return
		}
	}
	if stage.Target_id != "" {
		stage_info.Link.Enabled = stage.Link_enabled
		stage_info.Link.Target = stage.Target_id
		if stage.Source_dir != "" {
			stage_info.Link.SourceDir = stage.Source_dir
		}
		if stage.Target_dir != "" {
			stage_info.Link.TargetDir = stage.Target_dir
		}

	}

	if stage.Build_id != "" {
		stage_info.LastBuildStatus.BuildId = stage.Build_id
		stage_info.LastBuildStatus.Status = stage.Status
		stage_info.LastBuildStatus.Pod_name = stage.Pod_name
	}
	return
}

func FormatStageInfo(stage Stages) (stage_info Stage_info) {
	method := "models/FormatStage"
	stage_info.Metadata.Name = stage.Stage_name
	stage_info.Metadata.Id = stage.Stage_id
	stage_info.Metadata.CreationTime = stage.Creation_time
	stage_info.Metadata.Type = stage.Type
	if common.CUSTOM_STAGE_TYPE == stage.Type {
		stage_info.Metadata.CustomType = stage.Custom_type
	}
	stage_info.Spec.Ci.Enabled = stage.Ci_enabled
	if stage.Project_id != "" {
		stage_info.Spec.Project.Id = stage.Project_id
		stage_info.Spec.Project.Branch = stage.Default_branch
		stage_info.Spec.Project.RepoType = stage.Repo_type
	}
	if stage.Ci_config != "" {
		err := json.Unmarshal([]byte(stage.Ci_config), &stage_info.Spec.Ci.CiConfig)
		if err != nil {
			glog.Errorf("%s json unmarshal ci config failed: %v\n", method, err)
			return
		}
	}

	if stage.Container_info != "" {
		err := json.Unmarshal([]byte(stage.Container_info), &stage_info.Spec.Container)
		if err != nil {
			glog.Errorf("%s json unmarshal ci Container failed: %v\n", method, err)
			return
		}
	}

	stage_info.Spec.Container.Image = stage.Image
	if stage.Build_info != "" {
		err := json.Unmarshal([]byte(stage.Build_info), &stage_info.Spec.Build)
		if err != nil {
			glog.Errorf("%s json unmarshal ci Build_info failed:%v\n", method, err)
			return
		}
	}
	glog.Infof("stage.Target_id==%s\n", stage.Target_id)
	if stage.Target_id != "" {
		stage_info.Link.Enabled = stage.Link_enabled
		stage_info.Link.Target = stage.Target_id
		if stage.Source_dir != "" {
			stage_info.Link.SourceDir = stage.Source_dir
		}
		if stage.Target_dir != "" {
			stage_info.Link.TargetDir = stage.Target_dir
		}

	}

	if stage.Build_id != "" {
		stage_info.LastBuildStatus.BuildId = stage.Build_id
		stage_info.LastBuildStatus.Status = stage.Status
		stage_info.LastBuildStatus.Pod_name = stage.Pod_name
	}
	return
}

func (cs *CiStages) FindBuildEnabledStages(flowId string) (cistages []CiStages, total int64, err error) {
	o := orm.NewOrm()
	total, err = o.QueryTable(cs.TableName()).Filter("flow_id", flowId).All(&cistages)
	return
}

func (cs *CiStages) FindByProjectIdAndCI(projectId string, ci_enabled int) (cistages []CiStages, total int64, err error) {
	o := orm.NewOrm()
	total, err = o.QueryTable(cs.TableName()).Filter("project_id", projectId).
		Filter("ci_enabled", ci_enabled).All(&cistages)
	return
}
