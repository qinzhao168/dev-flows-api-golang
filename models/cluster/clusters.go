package cluster

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/golang/glog"

	"dev-flows-api-golang/models/common"
	sqlstatus "dev-flows-api-golang/models/sql/status"
)

// ClusterModel is the structure of a cluster both in mysql table and json output
type ClusterModel struct {
	ClusterID       string    `orm:"pk;column(id)" json:"clusterID"`
	ClusterName     string    `orm:"size(45);column(name)" json:"clusterName"`
	APIProtocol     string    `orm:"size(45);column(api_protocol)" json:"apiProtocol"`
	APIHost         string    `orm:"size(25);column(api_host)" json:"apiHost"`
	APIToken        string    `orm:"size(2048);column(api_token)" json:"apiToken"`
	APIVersion      string    `orm:"size(8);column(api_version)" json:"apiVersion"`
	Description     string    `orm:"size(2048)" json:"description"`
	PublicIPs       string    `orm:"size(256);column(public_ips)" json:"publicIPs"`
	BindingIPs      string    `orm:"column(binding_ips)" json:"bindingIPs"`
	BindingDomains  string    `orm:"size(2000);column(binding_domain)" json:"bindingDomains"`
	ConfigDetail    string    `orm:"size(2000);column(config_detail)" json:"configDetail"`
	Extention       string    `orm:"size(1024);column(extention)" json:"extention"`
	WebTerminal     string    `orm:"size(45);column(web_terminal_domain)" json:"webTerminal"`
	StorageID       string    `orm:"size(45);column(storage)" json:"storage_id"`
	CreationTime    time.Time `orm:"column(creation_time)" json:"creationTime"`
	IsDefault       int8      `orm:"column(is_default)" json:"isDefault"`
	ResourcePrice   string    `orm:"column(resource_price)" json:"resource_price"`
	Zone            string    `orm:"column(zone)" json:"zone"`
	QingcloudURL    string    `orm:"column(qingcloud_url)" json:"qingcloud_url"`
	AccessKeyID     string    `orm:"column(access_key_id)" json:"access_key_id"`
	SecretAccessKey string    `orm:"column(secret_access_key)" json:"secret_access_key"`
	Content         string    `orm:"column(content)" json:"-"`
	Type            int8      `orm:"column(type)" json:"type"` // 0 enncloud 1 huawei 2 aws ,default is 0
	ESUrl           string     `orm:"column(es_url)" json:"-"`
}

const (
	CanNotListNode           = 1
	CanListNodeByIP          = 2
	CanListNodeByLabels      = 3
	CanListNodeByIPAndLabels = 4
)

// Config ConfigDetail structure
type Config struct {
	ListNodes              uint8 `json:"listNodes, omitEmpty"`
	IsBuilder              uint8 `json:"isBuilder, omitEmpty"`
	NetworkPolicySupported *bool `json:"networkPolicySupported, omitEmpty"`
}

// TableName returns the name of table in database
func (cl *ClusterModel) TableName() string {
	return "tenx_clusters"
}

func NewClusterModel() *ClusterModel {
	return &ClusterModel{}
}
func jsonArrayToString(str string) (string, error) {
	if str == "" || str == "[]" || str == `[""]` {
		return "", nil
	}
	if !strings.Contains(str, "[") && !strings.Contains(str, "]") {
		return str, nil
	}
	array := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(str), &array); err != nil {
		return "", err
	}
	return strings.Join(array, ","), nil
}

// checkSliceFormString check if the *str is in josn []string form
// if not, make it happen, or return error
func checkSliceFormString(str *string) error {
	temp, err := jsonArrayToString(*str)
	if err != nil {
		return err
	}
	if temp != "" {
		temp = `["` + temp + `"]`
	}
	*str = temp
	return nil
}

// Validate make soure ClusterModel in a good form
func (cl *ClusterModel) Validate() error {
	var err error
	// PublicIPs
	if err = checkSliceFormString(&cl.PublicIPs); err != nil {
		return err
	}
	// BindingIPs
	if err = checkSliceFormString(&cl.BindingIPs); err != nil {
		return err
	}
	// BindingDomains
	if err = checkSliceFormString(&cl.BindingDomains); err != nil {
		return err
	}
	return nil
}

func (cl *ClusterModel) ListAll() ([]ClusterModel, error) {
	o := orm.NewOrm()
	var results []ClusterModel

	_, err := o.QueryTable(cl.TableName()).All(&results)
	return results, err
}

// List lists all teams with pagination
func (cl *ClusterModel) List(dataselect *common.DataSelectQuery) ([]ClusterModel, uint32, error) {
	o := orm.NewOrm()

	sql := fmt.Sprintf(`SELECT id FROM %s where %s %s %s;`, cl.TableName(), dataselect.FilterQuery, dataselect.SortQuery, dataselect.PaginationQuery)
	var clusterModels []ClusterModel
	_, err := o.Raw(sql).QueryRows(&clusterModels)
	if err != nil {
		errCode, err := sqlstatus.ParseErrorCode(err)
		return nil, errCode, err
	}

	if len(clusterModels) == 0 {
		return []ClusterModel{}, sqlstatus.SQLSuccess, nil
	}

	// get the cluster IDs
	var clusterIDStrs []string
	for _, item := range clusterModels {
		clusterIDStrs = append(clusterIDStrs, fmt.Sprintf("'%s'", item.ClusterID))
	}

	sql = fmt.Sprintf(`SELECT
    *
FROM
    %s
WHERE
    id IN (%s) %s;`, cl.TableName(), strings.Join(clusterIDStrs, ","), dataselect.SortQuery)
	_, err = o.Raw(sql).QueryRows(&clusterModels)
	errCode, err := sqlstatus.ParseErrorCode(err)
	return clusterModels, errCode, err
}

// ListByIDs lists all clusters by cluster IDs
func (cl *ClusterModel) ListByIDs(clusterIDs []string, dataselect *common.DataSelectQuery) ([]ClusterModel, uint32, error) {
	o := orm.NewOrm()

	clusterIDStr := fmt.Sprintf("'%s'", strings.Join(clusterIDs, "','"))
	sql := fmt.Sprintf(`SELECT * FROM %s WHERE id IN (%s) %s %s;`, cl.TableName(), clusterIDStr,
		dataselect.SortQuery, dataselect.PaginationQuery)
	var clusterModels []ClusterModel
	_, err := o.Raw(sql).QueryRows(&clusterModels)
	errCode, err := sqlstatus.ParseErrorCode(err)
	return clusterModels, errCode, err
}

// Count counts all clusters
func (cl *ClusterModel) Count(dataselect *common.DataSelectQuery) (int, uint32, error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf(`SELECT  count(*) as total FROM %s where %s;`, cl.TableName(), dataselect.FilterQuery)
	meta := &common.ListMeta{}
	err := o.Raw(sql).QueryRow(meta)
	if err != nil {
		errCode, err := sqlstatus.ParseErrorCode(err)
		return 0, errCode, err
	}
	return meta.Total, sqlstatus.SQLSuccess, nil
}

// Get consumes a cluster id, and load cluster info to this pointer,  returns status code findClusterById
func (cl *ClusterModel) Get(ID string) (uint32, error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where id=?;", cl.TableName())
	err := o.Raw(sql, ID).QueryRow(cl)

	return sqlstatus.ParseErrorCode(err)
}

// ListDefaultClusters returns all default clusters
func (cl *ClusterModel) ListDefaultClusters() ([]ClusterModel, uint32, error) {
	o := orm.NewOrm()
	var clusters []ClusterModel
	_, err := o.QueryTable(cl.TableName()).Filter("is_default", 1).All(&clusters)
	errcode, err := sqlstatus.ParseErrorCode(err)
	return clusters, errcode, err
}

func (cl *ClusterModel) CheckClusterAuthorized(clusterId string) bool {
	o := orm.NewOrm()
	return o.QueryTable(cl.TableName()).Filter("is_default", 1).Filter("id", clusterId).Exist()
}

// GetByName consumes a cluster name, and load cluster info to this pointer,  returns status code
func (cl *ClusterModel) GetByName(name string) (uint32, error) {
	o := orm.NewOrm()
	sql := fmt.Sprintf("select * from %s where name=?;", cl.TableName())
	err := o.Raw(sql, name).QueryRow(cl)
	return sqlstatus.ParseErrorCode(err)
}

// Insert a new cluster record with clusterID already determined
func (cl *ClusterModel) Insert() (int64, error) {
	if err := cl.Validate(); err != nil {
		return 0, err
	}
	o := orm.NewOrm()
	cl.CreationTime = time.Now()

	switch common.DType {
	case common.DRMySQL:
		count, err := o.Insert(cl)
		return count, err
	case common.DRPostgres:
		sql := `INSERT INTO "tenx_clusters" ("id", "name", "api_protocol", "api_host", "api_token", "api_version", "description", "creation_time", "public_ips", "binding_domain", "config_detail", "web_terminal_domain", "is_default") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
		_, err := o.Raw(sql, cl.ClusterID, cl.ClusterName, cl.APIProtocol, cl.APIHost, cl.APIToken, cl.APIVersion, cl.Description, cl.CreationTime, cl.PublicIPs, cl.BindingDomains, cl.ConfigDetail, cl.WebTerminal, cl.IsDefault).Exec()
		return 0, err
	}

	return 0, fmt.Errorf("Driver %s not supported", common.DType)
}

// Delete deletes a cluster record and returns the status code
func (cl *ClusterModel) Delete() error {
	o := orm.NewOrm()

	_, err := o.Delete(cl)
	return err
}

// Update updates one or more fields for one time
func (cl *ClusterModel) Update(cols ...string) error {
	err := cl.Validate()
	if err != nil {
		return err
	}
	o := orm.NewOrm()

	_, err = o.Update(cl, cols...)
	return err
}

func UpdateClusterStorage(storageID string) error {
	sql := "update tenx_clusters set storage = ?"
	o := orm.NewOrm()
	_, err := o.Raw(sql, storageID).Exec()
	return err
}

// GetStorageDetail get storage detail by tenx_clusters and tenx_configs
func (cl *ClusterModel) GetStorageDetail(clusterID string) (string, error) {
	method := "Cluster.GetStorageDetail"
	var qb orm.QueryBuilder
	var err error
	switch common.DType {
	case common.DRMySQL:
		qb, err = orm.NewQueryBuilder("mysql")
	case common.DRPostgres:
		qb, err = orm.NewQueryBuilder("postgres")
	}
	if err != nil {
		glog.Errorln(method, "orm.NewQueryBuilder failed. err:", err)
		return "", err
	}
	qb.Select("tenx_configs.config_detail as detail").
		From("tenx_configs").
		InnerJoin("tenx_clusters").On("tenx_configs.config_id = tenx_clusters.storage").
		Where("tenx_clusters.id = ?")
	sql := qb.String()
	o := orm.NewOrm()

	config := struct {
		Detail string
	}{}
	err = o.Raw(sql, clusterID).QueryRow(&config)
	if err != nil {
		glog.Errorln(method, "orm.QueryRow failed. err:", err)
		return "", err
	}
	return config.Detail, nil
}

// GetPublicIPs get public ips from db
func (cl ClusterModel) GetPublicIPs(ID string) ([]string, error) {
	o := orm.NewOrm()
	err := o.QueryTable(cl.TableName()).Filter("id", ID).One(&cl, "PublicIPs")
	if err == orm.ErrNoRows {
		return []string{}, nil
	}
	if err != nil {
		glog.Errorf("Get public ips fails, cluster id:%s, error:%v\n", ID, err)
		return []string{}, err
	}
	ips := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(cl.PublicIPs), &ips); err != nil {
		glog.Errorf("Parse public ips %s fails, cluster id:%s, error:%v\n", cl.PublicIPs, ID, err)
		return ips, err
	}

	return ips, nil
}

func (t *ClusterModel) GetClusterByTeamID(teamID string) ([]ClusterModel, error) {
	method := "GetClusterByTeamID"
	var result []ClusterModel
	o := orm.NewOrm()
	sql := `SELECT * FROM tenx_team_resource_ref AS t1 INNER JOIN tenx_clusters AS t2
	ON t1.resource_id=t2.id
	WHERE t1.resource_type = 0 AND t1.team_id = ?`
	if _, err := o.Raw(sql, teamID).QueryRows(&result); err != nil {
		glog.Errorln(method, "failed.", err)
		return nil, err
	}
	return result, nil
}

//GetAllCluster
func (t *ClusterModel) GetAllCluster() ([]ClusterModel, error) {
	method := "GetAllCluster"
	var result []ClusterModel
	o := orm.NewOrm()
	sql := `SELECT * FROM tenx_clusters`
	if _, err := o.Raw(sql).QueryRows(&result); err != nil {
		glog.Errorln(method, "failed.", err)
		return nil, err
	}
	return result, nil
}
