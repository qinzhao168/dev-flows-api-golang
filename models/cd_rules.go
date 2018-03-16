package models

import (
	"time"
	"github.com/astaxie/beego/orm"
	"fmt"
	"github.com/golang/glog"
)

type CDRules struct {
	RuleId                string `orm:"pk;column(rule_id)" json:"rule_id"`
	Namespace             string `orm:"column(namespace)" json:"namespace"`
	FlowId                string `orm:"column(flow_id)" json:"flow_id"`
	ImageName             string `orm:"column(image_name)" json:"image_name"`
	BindingClusterId      string `orm:"column(binding_cluster_id)" json:"binding_cluster_id"`
	BindingDeploymentId   string `orm:"column(binding_deployment_id)" json:"binding_deployment_id"`
	BindingDeploymentName string `orm:"column(binding_deployment_name)" json:"binding_deployment_name"`
	UpgradeStrategy       int8 `orm:"column(upgrade_strategy)" json:"upgrade_strategy"` //升级类型
	MatchTag              string `orm:"column(match_tag)" json:"match_tag"`
	Enabled               int8 `orm:"column(enabled)" json:"enabled"`
	CreateTime            time.Time `orm:"column(create_time)" json:"create_time"`
	UpdateTime            time.Time `orm:"column(update_time)" json:"update_time"`
	Tag                   string `orm:"column(tag)" json:"tag,omitempty"`
	MinReadySeconds       int32 `orm:"column(min_ready_seconds)" json:"min_ready_seconds"`
}

type CdRuleReq struct {
	FlowId           string `json:"flowId"`
	Image_name       string `json:"image_name"`
	Match_tag        string `json:"match_tag"`
	Upgrade_strategy int8 `json:"upgrade_strategy"`
	MinReadySeconds  int32 `json:"min_ready_seconds"`
	Binding_service  Binding_service `json:"binding_service"`
}

type Binding_service struct {
	Cluster_id      string `json:"cluster_id"`
	Deployment_id   string `json:"deployment_id"`
	Deployment_name string `json:"deployment_name"`
}

type CdRuleResp struct {
	Rule_id string `json:"rule_id"`
	Message string `json:"message"`
}

func (cd *CDRules) TableName() string {

	return "tenx_cd_rules"

}

func NewCdRules() *CDRules {

	return &CDRules{}

}

func (cd *CDRules) FindEnabledRuleByImage(imageName string, orms ...orm.Ormer) (rules []CDRules, result int64, err error) {

	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	glog.Infof("imageFullName=%s\n", imageName)
	sql := fmt.Sprintf("select * from %s where image_name=? and enabled=?", cd.TableName())

	result, err = o.Raw(sql, imageName, 1).QueryRows(&rules)
	return
}

func (cd *CDRules) FindDeploymentCDRule(namespace, cluster, name string, orms ...orm.Ormer) ([]*CDRules, int64, error) {
	var cdRules []*CDRules
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	sql := fmt.Sprintf("select * from %s where namespace=? and binding_cluster_id=? and binding_deployment_name in (?)", cd.TableName())
	total, err := o.Raw(sql, namespace, cluster, name).QueryRows(&cdRules)
	return cdRules, total, err
}

func (cd *CDRules) DeleteDeploymentCDRule(namespace, cluster, name string, orms ...orm.Ormer) error {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	sql := fmt.Sprintf("delete from %s where namespace=? and binding_cluster_id=? and binding_deployment_name in (?)", cd.TableName())
	p, err := o.Raw(sql).Prepare()
	if err != nil {
		return err
	}
	_, err = p.Exec(namespace, cluster, name)
	if err != nil {
		return err
	}
	defer p.Close()

	return nil
}

func (cd *CDRules) UpdateCDRule(namespace, flow_id, rule_id string, rule CDRules, orms ...orm.Ormer) (int64, error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	return o.Update(&rule, "update_time", "binding_cluster_id", "binding_deployment_id", "binding_deployment_name",
		"image_name", "flow_id", "upgrade_strategy", "match_tag", "min_ready_seconds")
}

func (cd *CDRules) ListRulesByFlowId(namespace, flow_id string, orms ...orm.Ormer) (cdRules []CDRules, total int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	total, err = o.QueryTable(cd.TableName()).Filter("namespace", namespace).
		Filter("flow_id", flow_id).Filter("enabled", 1).All(&cdRules)
	return
}

func (cd *CDRules) ListRulesByFlowIdAndImageName(image_name, namespace string, orms ...orm.Ormer) (cdRules []CDRules, total int64, err error) {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}
	sql := fmt.Sprintf("select * from %s where image_name=? and enabled=? and namespace!=?", cd.TableName())

	total, err = o.Raw(sql, image_name, 1, namespace).QueryRows(&cdRules)
	//total, err = o.QueryTable(cd.TableName()).
	//	Filter("image_name", image_name).Filter("enabled", 1).All(&cdRules)
	return
}

func IsValidRule(rule CdRuleReq) bool {
	method := "IsValidRule"
	if rule.FlowId == "" || rule.Image_name == "" || rule.Match_tag == "" || (rule.Upgrade_strategy != 1 && rule.Upgrade_strategy != 2) {
		glog.Errorf("%s %s \n", method, "Invalid flow_id, image_name, tag, binding_service or upgrade_strategy")
		return false
	}

	if rule.Binding_service.Cluster_id == "" || rule.Binding_service.Deployment_id == "" || rule.Binding_service.Deployment_name == "" {
		glog.Errorf("%s %s \n", method, "Invalid binding_cluster_id, binding_deployment_id or binding_deployment_name")
		return false
	}
	return true
}

func (cd *CDRules) FindMatchingRule(namespace, flow_id, image_name, match_tag, clusterId, deploymentName string) (cdRules CDRules, err error) {
	o := orm.NewOrm()
	err = o.QueryTable(cd.TableName()).Filter("namespace", namespace).
		Filter("flow_id", flow_id).Filter("enabled", 1).
		Filter("image_name", image_name).Filter("match_tag", match_tag).
		Filter("binding_cluster_id", clusterId).Filter("binding_deployment_name", deploymentName).One(&cdRules)
	return
}

func (cd *CDRules) CreateOneRule(rule CDRules) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.Insert(&rule)
	return
}

//func (cd *CDRules)DeleteDeploymentCDRule(namespace, cluster, name string) (result int64, err error) {
//	o := orm.NewOrm()
//	result, err = o.QueryTable(cd.TableName()).Filter("namespace", namespace).
//		Filter("binding_cluster_id", cluster).Filter("binding_deployment_name", name).Delete()
//	return
//}

func (cd *CDRules) RemoveRule(namespace, flow_id, rule_id string) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.QueryTable(cd.TableName()).Filter("namespace", namespace).
		Filter("flow_id", flow_id).Filter("rule_id", rule_id).Update(orm.Params{
		"enabled": 0,
	})
	return
}

func (cd *CDRules) UpdateRuleById(namespace, flow_id, rule_id string, rule CDRules) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.QueryTable(cd.TableName()).Filter("namespace", namespace).
		Filter("flow_id", flow_id).Filter("rule_id", rule_id).Update(orm.Params{
		"enabled":                 rule.Enabled,
		"image_name":              rule.ImageName,
		"binding_cluster_id":      rule.BindingClusterId,
		"binding_deployment_id":   rule.BindingDeploymentId,
		"binding_deployment_name": rule.BindingDeploymentName,
		"upgrade_strategy":        rule.UpgradeStrategy,
		"match_tag":               rule.MatchTag,
		"update_time":             time.Now(),
	})
	return
}
