package models

import (
	"github.com/astaxie/beego/orm"
	"time"
	"fmt"
)

type CiCrontab struct {
	FlowId        string `orm:"pk;column(flow_id)" json:"flow_id"`
	CrontabId     int `orm:"column(crontabId)" json:"crontabId"`
	Enabled       int `orm:"column(enabled)" json:"enabled"`
	DoCrontabTime time.Time `orm:"column(doCrontabTime)" json:"doCrontabTime"`
}

func (cd *CiCrontab) TableName() string {

	return "tenx_ci_crontab"

}

func NewCiCrontab() *CiCrontab {

	return &CiCrontab{}

}

func (cd *CiCrontab) DeleteAllCiCrontab(orms ...orm.Ormer) error {
	var o orm.Ormer
	if len(orms) != 1 {
		o = orm.NewOrm()
	} else {
		o = orms[0]
	}

	sql := fmt.Sprintf("delete from %s", cd.TableName())
	p, err := o.Raw(sql).Prepare()
	if err != nil {
		return err
	}
	_, err = p.Exec()
	if err != nil {
		return err
	}
	defer p.Close()

	return nil
}

func (cd *CiCrontab) FindCiCrontabByFlowId(flow_id string) (ciCrontab CiCrontab, err error) {
	o := orm.NewOrm()
	err = o.QueryTable(cd.TableName()).
		Filter("flow_id", flow_id).One(&ciCrontab)
	return
}

func (cd *CiCrontab) Exist(flow_id string) bool {
	o := orm.NewOrm()

	return o.QueryTable(cd.TableName()).
		Filter("flow_id", flow_id).Exist()
}

func (cd *CiCrontab) CreateOneCiCrontab(crontab CiCrontab) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.Insert(&crontab)
	return
}

func (cd *CiCrontab) EnabledCiCrontab(flow_id string, doCrontabTime time.Time, enabled int) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.QueryTable(cd.TableName()).
		Filter("flow_id", flow_id).Update(orm.Params{
		"enabled":       enabled,
		"doCrontabTime": doCrontabTime,
	})
	return
}

func (cd *CiCrontab) UpdateCiCrontabByFlowId(flow_id string, doCrontabTime time.Time, crontabId, enabled int) (result int64, err error) {
	o := orm.NewOrm()
	result, err = o.QueryTable(cd.TableName()).
		Filter("flow_id", flow_id).Update(orm.Params{
		"enabled":       enabled,
		"crontabId":     crontabId,
		"doCrontabTime": doCrontabTime,
	})
	return
}
