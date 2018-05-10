package coderepo

import (
	//"github.com/satori/go.uuid"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	gitlab "github.com/drone/drone/remote/gitlab3/client"
	"dev-flows-api-golang/util/rand"
	"dev-flows-api-golang/models/common"
	"strconv"
	"strings"
	"github.com/golang/glog"
	"time"
)

//const DEFAULT_PAGE_SIZE  = 50
type GitlabClient struct {
	Gitlab_url   string `json:"gitlab_url"`
	Access_token string `json:"access_token"`
	Gitlab       *gitlab.Client
}

var _ RepoApier = &GitlabClient{}

//https://docs.gitlab.com/ce/api/projects.html

func NewGitlabClient(Gitlab_url, Access_token string) *GitlabClient {
	method := "NewGitlabClient"
	if Gitlab_url == "" || Access_token == "" {
		return nil
	}
	glog.Infof("method=%s\n", Gitlab_url)
	droneGitlab := Gitlab_url

	glog.Infof("%s gitlabUrl = %s", method, droneGitlab)

	if strings.Index(Gitlab_url, "/api/v3") < 0 {
		Gitlab_url += "/api/v3"
	}

	return &GitlabClient{
		Gitlab_url:   Gitlab_url,
		Access_token: Access_token,
		Gitlab:       gitlab.New(droneGitlab, "/api/v3", Access_token, true),
	}

}

func (g *GitlabClient) GetUrl(endpoint string, querys map[string]int) (string, error) {
	querysString := ""
	if endpoint == "" {
		return "", errors.New("endpoint is null")
	}
	if len(querys) != 0 {
		for q, v := range querys {
			querysString += "&" + q + "=" + fmt.Sprintf("%d", v)
		}
	}
	reqUrl := g.Gitlab_url + endpoint + "?private_token=" + g.Access_token + querysString

	return reqUrl, nil
}

func (g *GitlabClient) GetUserInfo() (UserInfo, error) {
	var userInfo UserInfo
	url, err := g.GetUrl("/user", nil)
	if err != nil {
		return userInfo, err
	}
	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return userInfo, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return userInfo, err
	}

	err = json.Unmarshal(data, &userInfo)
	if err != nil {
		return userInfo, err
	}
	userInfo.Type = "user"
	userInfo.Url = g.Gitlab_url + "/" + userInfo.Username
	return userInfo, nil

}

func (g *GitlabClient) GetUserAndOrgs() (userInfos []UserInfo, err error) {
	user, err := g.GetUserInfo()
	if err != nil {
		glog.Errorf("GetUserAndOrgs failed:%v\n", err)
	}

	userInfos = make([]UserInfo, 0)

	userInfos = append(userInfos, user)
	return
}

func (g *GitlabClient) GetUserOrgs() ([]UserInfo, error) {
	//return g.GetUserInfo()
	return nil, nil
}

// Get a list of users.
func (g *GitlabClient) GetAllUsers() ([]UserInfo, error) {
	var userInfos []UserInfo
	url, err := g.GetUrl("/users", nil)
	if err != nil {
		return userInfos, err
	}
	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return userInfos, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return userInfos, err
	}

	err = json.Unmarshal(data, &userInfos)
	if err != nil {
		return userInfos, err
	}
	if len(userInfos) != 0 {
		for _, userInfo := range userInfos {
			userInfo.Type = "user"
			userInfo.Url = g.Gitlab_url + "/" + userInfo.Username
		}
	}
	return userInfos, nil
}

//[
//{
//"clone_url" : "http://10.39.0.53:8880/weihongwei/xinzhiyun.git",
//"description" : "",
//"name" : "weihongwei/xinzhiyun",
//"owner" : {
//"avatar_url" : "http://www.gravatar.com/avatar/e7d47e7992b977cb3f0938252205b99c?s=80&d=identicon",
//"id" : 2,
//"name" : "weihongwei",
//"state" : "active",
//"username" : "weihongwei",
//"web_url" : "http://10.39.0.53:8880/weihongwei"
//},
//"private" : true,
//"projectId" : 2,
//"ssh_url" : "git@10.39.0.53:weihongwei/xinzhiyun.git",
//"url" : "http://10.39.0.53:8880/weihongwei/xinzhiyun"
//}
//]
//[{"clone_url":"http://10.39.0.53:8880/root/demo.git"
//,"description":"","name":"Administrator / demo",
//"owner":{"name":"Administrator","id":1,"state":"active",
//"avatar_url":"http://www.gravatar.com/avatar/e64c7d89f26bd1972efa854d13d7dd61?s=80\u0026d=identicon",
//"username":"root","web_url":"http://10.39.0.53:8880/root"},
//"private":true,"projectId":1,
//"ssh_url":"git@10.39.0.53:root/demo.git",
//"url":"http://10.39.0.53:8880/root/demo",
//"managed_project":{"active":0,"id":""}}]

func (g *GitlabClient) GetAllUsersRepos(userinfos []UserInfo) ([]ReposGitHubAndGogs, error) {
	var repos []ReposGitHubAndGogs
	var repo ReposGitHubAndGogs
	repos = make([]ReposGitHubAndGogs, 0)

	projects, err := g.Gitlab.AllProjects(false)
	if err != nil {
		return repos, err
	}
	glog.Infof("gitlab projects=%s\n", projects)
	for _, project := range projects {
		glog.Infof("gitlab project=%s\n", project)
		repo.CloneUrl = project.HttpRepoUrl
		repo.Description = project.Description
		repo.Name = project.NameWithNamespace
		repo.Private = !project.Public
		repo.ProjectId = project.Id
		repo.SshUrl = project.SshRepoUrl
		repo.Url = project.Url
		repo.Owner.Id = project.Owner.Id
		repo.Owner.Name = project.Owner.Name
		repo.Owner.Username = project.Owner.Username
		repo.Owner.WebUrl = project.Owner.WebUrl
		repo.Owner.State = project.Owner.State
		repo.Owner.Avatar_url = project.Owner.AvatarUrl
		repos = append(repos, repo)
	}

	glog.Infof("gitlab repos=%s\n", repos)
	return repos, nil
}

// Get a list of projects accessible by the authenticated user.
func (g *GitlabClient) GetUserRepos() {

}

func (g *GitlabClient) createAllRequestForRepo(endpoint string) ([]RepoGitLab, error) {
	repoGitLab := make([]RepoGitLab, 0)

	querys := map[string]int{
		"page":      1,
		"page_size": DEFAULT_PAGE_SIZE,
	}

	url, err := g.GetUrl(endpoint, querys)
	if err != nil {
		return repoGitLab, err
	}

	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return repoGitLab, err
	}
	defer resp.Body.Close()
	totalPagesA, err := strconv.Atoi(resp.Header.Get("x-total-pages"))
	if err != nil {
		return repoGitLab, err
	}

	if totalPagesA < 2 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return repoGitLab, err
		}
		glog.Infof("repo data:%s\n", string(data))
		err = json.Unmarshal(data, &repoGitLab)
		if err != nil {
			return repoGitLab, err
		}

		return repoGitLab, nil

	} else {
		for ; totalPagesA > 1; totalPagesA-- {
			querys["page"] += 1
			urlPage, err := g.GetUrl(endpoint, querys)
			if err != nil {
				return repoGitLab, err
			}
			respPage, err := HttpClientRequest("GET", urlPage, nil, nil)
			if err != nil {
				respPage.Body.Close()
				return repoGitLab, err
			}
			dataPage, err := ioutil.ReadAll(respPage.Body)
			if err != nil {
				respPage.Body.Close()
				return repoGitLab, err
			}
			gitRepo := make([]RepoGitLab, 0)
			glog.Infof("repo data:%s\n", string(dataPage))
			err = json.Unmarshal(dataPage, &gitRepo)
			if err != nil {
				respPage.Body.Close()
				return repoGitLab, err
			}
			copy(repoGitLab, gitRepo)
		}
	}

	return repoGitLab, nil

}

func (g *GitlabClient) GetUserAllrepos() {

}

func (g *GitlabClient) GetUserOwnRepo() {
	//var hooks []WebhookResp
	//url, err := g.GetUrl(fmt.Sprintf("/projects/%d/hooks",projectId), nil)
	//if err != nil {
	//	return hooks, err
	//}
	//
	//resp, err := HttpClientRequest("GET", url, nil, nil)
	//if err != nil {
	//	return hooks, err
	//}
	//
	//err = ReadBody(resp, &hooks)
	//if err != nil {
	//	return hooks, err
	//}
	//
	//return hooks, nil
}

type GitLabBranch struct {
	Branch string `json:"name"`
	Commit Commit `json:"commit"`
}

//[{"name":"master","commit":{"id":"547a32660b42256e5ec02e9ac8589a8329d5d11e",
//"short_id":"547a3266","title":"Update README.md","created_at":"2017-08-02T15:19:49.000+08:00",
//"parent_ids":["3b94d14ea8586afceb4414191018373604f7422f"],"message":"Update README.md",
//"author_name":"Administrator","author_email":"admin@example.com",
//"authored_date":"2017-08-02T15:19:49.000+08:00","committer_name":"Administrator",
//"committer_email":"admin@example.com","committed_date":"2017-08-02T15:19:49.000+08:00"},
//"merged":false,"protected":true,"developers_can_push":false,"developers_can_merge":false}]
func (g *GitlabClient) GetRepoAllBranches(repoName string, repoId int) ([]BranchResp, error) {
	var branchs []BranchResp
	branchs = make([]BranchResp, 0)
	url, err := g.GetUrl(fmt.Sprintf("/projects/%d/repository/branches", repoId), nil)
	if err != nil {
		return branchs, err
	}

	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return branchs, err
	}
	var gitlabBranchList []GitLabBranch
	err = ReadBody(resp, &gitlabBranchList)
	if err != nil {
		return branchs, err
	}

	var branch BranchResp
	for _, lab := range gitlabBranchList {
		branch.Branch = lab.Branch
		branch.CommitId = lab.Commit.Id
		branch.CommitterName = lab.Commit.Committer_name
		branch.Message = lab.Commit.Message
		branch.CommittedDate = lab.Commit.Committed_date
		branchs = append(branchs, branch)

	}
	return branchs, nil

}

//[{"name":"demo","message":"","commit":{"id":"02a71fc39773dc60f1ae8e5282be2350476b7b8d",
//"message":"Add readme.md","parent_ids":[],"authored_date":"2017-11-04T08:09:20.000+08:00",
//"author_name":"Administrator","author_email":"admin@example.com",
//"committed_date":"2017-11-04T08:09:20.000+08:00","committer_name":"Administrator",
//"committer_email":"admin@example.com"},"release":{"tag_name":"demo","description":"的"}}]

type GitLabTags struct {
	Tag         string `json:"name"`
	Description string `json:"message"`
	Commit      Commit `json:"commit"`
}

func (g *GitlabClient) GetRepoAllTags(repoName string, repoId int) ([]Tag, error) {

	var tags []Tag
	tags = make([]Tag, 0)
	url, err := g.GetUrl(fmt.Sprintf("/projects/%d/repository/tags", repoId), nil)
	if err != nil {
		return tags, err
	}

	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return tags, err
	}
	var gitlabTags []GitLabTags
	err = ReadBody(resp, &gitlabTags)
	if err != nil {
		return tags, err
	}
	var tag Tag
	for _, lab := range gitlabTags {
		tag.Tag = lab.Tag
		tag.Description = lab.Description
		tag.Commit_id = lab.Commit.Id
		tag.CommitterName = lab.Commit.Committer_name
		tag.Message = lab.Commit.Message
		tag.Committed_date = lab.Commit.Committed_date
		tags = append(tags, tag)

	}
	return tags, nil
}

//repoFullName is gitlab_project_id
func (g *GitlabClient) GetOneWebhook(repoFullName, webHookId string) (WebhookResp, error) {
	var hook WebhookResp
	url, err := g.GetUrl(fmt.Sprintf("/projects/%s/hooks/%s", repoFullName, webHookId), nil)
	if err != nil {
		return hook, err
	}

	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return hook, err
	}

	err = ReadBody(resp, &hook)
	if err != nil {
		return hook, err
	}

	return hook, nil
}

func (g *GitlabClient) GetProjectWebhooks(projectId int) ([]WebhookResp, error) {
	var hooks []WebhookResp
	url, err := g.GetUrl(fmt.Sprintf("/projects/%d/hooks", projectId), nil)
	if err != nil {
		return hooks, err
	}

	resp, err := HttpClientRequest("GET", url, nil, nil)
	if err != nil {
		return hooks, err
	}

	err = ReadBody(resp, &hooks)
	if err != nil {
		return hooks, err
	}

	return hooks, nil
}

func (g *GitlabClient) RemoveDeployKey(proId, keyId int, repoName string) error {
	endpoint := fmt.Sprintf("/projects/%d/keys/%d", proId, keyId)
	url, err := g.GetUrl(endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := HttpClientRequest("DELETE", url, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {

		return err

	}

	return nil
}

func (g *GitlabClient) UpdateWebHook(projectId, repo_full_name string, events []string) (WebhookResp, error) {
	return WebhookResp{}, nil
}

type GitLapWebHook struct {
	Id                    int `json:"id"`
	Url                   string `json:"url"`
	PushEvents            bool `json:"push_events"`
	TagPushEvents         bool `json:"tag_push_events"`
	IssuesEvents          bool `json:"issues_events"`
	MergeRequestsEvents   bool `json:"merge_requests_events"`
	NoteEvents            bool `json:"note_events"`
	EnableSslVerification bool `json:"enable_ssl_verification"`
	CreatedAt             time.Time `json:"created_at"`
}

//{"id":24,"url":"http://localhost:8090/api/v2/devops/managed-projects/webhooks/MPID-xMHroxHfcFK2",
//"created_at":"2017-11-03T14:25:34.513Z","push_events":true,
//"tag_push_events":true,"repository_update_events":false,"enable_ssl_verification":true,
//"project_id":9,"issues_events":false,"merge_requests_events":true,"note_events":false,
//"pipeline_events":false,"wiki_page_events":false,"build_events":false}

func (g *GitlabClient) CreateWebhook(projectId string, events Event, repoName string) (WebhookResp, error) {
	method := "GitlabClientCreateWebhook"
	var webhook WebhookResp

	endpoint := fmt.Sprintf("/projects/%s/hooks", repoName)
	url, err := g.GetUrl(endpoint, nil)
	if err != nil {
		return webhook, err
	}

	hookUrl := common.WebHookUrlPrefix + projectId
	//如果只生成webhook
	if events.Only_gen_webhook {
		webhook.Url = hookUrl
		return webhook, nil
	}

	var reqOptions GitLapWebHook
	id, _ := strconv.Atoi(repoName) //projectId
	reqOptions.Id = id
	reqOptions.Url = hookUrl
	reqOptions.PushEvents = true
	reqOptions.TagPushEvents = true
	reqOptions.IssuesEvents = false
	reqOptions.MergeRequestsEvents = true
	reqOptions.NoteEvents = false
	reqOptions.EnableSslVerification = true
	reqOptions.CreatedAt = time.Now()

	reqWebhook, err := json.Marshal(reqOptions)
	if err != nil {
		return webhook, err
	}

	resp, err := HttpClientRequest("POST", url, strings.NewReader(string(reqWebhook)), nil)
	if err != nil {
		return webhook, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return webhook, err
	}
	glog.Infof("%s CreateWebhook webhook info:%s\n", method, string(data))
	err = json.Unmarshal(data, &webhook)
	if err != nil {
		return webhook, err
	}
	return webhook, nil

}

func (g *GitlabClient) RemoveWebhook(projectId string, hook_id int, repoName string) error {
	endpoint := fmt.Sprintf("/projects/%s/hooks/%d", projectId, hook_id)
	url, err := g.GetUrl(endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := HttpClientRequest("DELETE", url, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {

		return err

	}

	return nil
}

func (g *GitlabClient) CreateAllRequestForRepo() {

}

func (g *GitlabClient) FormatUser() {

}

func (g *GitlabClient) FomateWebhook() {

}
func (g *GitlabClient) FormatTag() {

}

func (g *GitlabClient) AddDeployKey(projectId, publicKey, repoName string) (AddDeployResp, error) {
	var key AddDeployResp
	url, err := g.GetUrl(fmt.Sprintf("/projects/%s/keys", projectId), nil)
	if err != nil {
		return key, err
	}

	deployReqData := AddDeployReq{
		Title: fmt.Sprintf("qinzhao@ennew.cn-%s", rand.RandString(5)),
		Key:   publicKey,
	}

	deployKey, err := json.Marshal(deployReqData)
	if err != nil {
		return key, err
	}

	resp, err := HttpClientRequest("POST", url, strings.NewReader(string(deployKey)), nil)
	if err != nil {
		return key, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return key, err
	}

	err = json.Unmarshal(data, &key)
	if err != nil {
		return key, err
	}
	return key, nil

}
