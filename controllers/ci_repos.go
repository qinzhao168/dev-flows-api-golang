package controllers

import (
	"dev-flows-api-golang/ci/coderepo"
	"os"
	"github.com/satori/go.uuid"
	"fmt"
	"dev-flows-api-golang/models"
	//gogsClient "github.com/gogits/go-gogs-client"
	sqlstatus "dev-flows-api-golang/models/sql/status"
	"github.com/golang/glog"
	"encoding/json"
	"strings"
	"strconv"
	"time"
	"net/http"
)

const GITHUB = "github"
const GITLAB = "gitlab"
const GOGS = "gogs"
const SVN = "svn"

type CiReposController struct {
	BaseController
}

type GogsGithubRepoResp struct {
	Repos []coderepo.ReposGitHubAndGogs `json:"repos"`
	User  string `json:"user"`
}

// @Title Update
// @Description update the object
// @Param	objectId		path 	string	true		"The objectid you want to update"
// @Param	body		body 	models.Object	true		"The body"
// @Success 200 {object} models.Object
// @Failure 403 :objectId is empty
// @router /:type [get]
func (cirepo *CiReposController) GetRepositories() {
	method := "CiReposController/GetRepositories"
	repoType := cirepo.Ctx.Input.Param(":type")
	scmUser := cirepo.GetString("user")

	depot := models.NewCiRepos()
	err := depot.FindOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		glog.Errorf(" %s get repo info from database failed: %v\n", method, err)
		cirepo.ResponseErrorAndCode("No repository found for the "+repoType, http.StatusOK)
		return
	}
	//gogs gihub resp
	var gogsGitHubResp = make(map[string]GogsGithubRepoResp)
	//gogs github accept json data info
	var repoListGogsGitHub = make(map[string][]coderepo.ReposGitHubAndGogs)
	//gitlab resp
	//glog.Infof("%s depot.RepoList=%v",method,depot.RepoList)
	var repoListGitLab []coderepo.ReposGitHubAndGogs
	if repoType == GOGS || repoType == GITLAB || repoType == GITHUB {
		if repoType != GITLAB {
			err = json.Unmarshal([]byte(depot.RepoList), &repoListGogsGitHub)
			if err != nil {
				glog.Errorf(" %s github gogs json unmarshal failed: %v\n", method, err)
				cirepo.ResponseErrorAndCode(" github gogs json unmarshal failed:", http.StatusInternalServerError)
				return
			}

			var reposUser GogsGithubRepoResp
			// If user specified, return the matched one
			if scmUser != "" {

				for user, repos := range repoListGogsGitHub {
					if user == scmUser {
						reposUser.User = user
						reposUser.Repos = repos
						gogsGitHubResp[user] = reposUser
					}
				}

			} else {

				for user, repos := range repoListGogsGitHub {
					reposUser.User = user
					reposUser.Repos = repos
					gogsGitHubResp[user] = reposUser
				}

			}

		} else {
			//gitlab
			err = json.Unmarshal([]byte(depot.RepoList), &repoListGitLab)
			if err != nil {
				glog.Errorf(" %s gitlab json unmarshal failed: %v\n", method, err)
				cirepo.ResponseErrorAndCode("gitlab json unmarshal failed:", http.StatusInternalServerError)
				return
			}

		}

		//显示激活
		mamageProjectList, total, err := models.NewCiManagedProjects().ListProjectsByType(cirepo.Namespace, repoType)
		if err != nil {
			glog.Errorf(" %s get manageProject list %s failed: %v\n", method, cirepo.Namespace, err)
			cirepo.ResponseErrorAndCode(" get manageProject list namespace=["+cirepo.Namespace+"] failed:"+fmt.Sprintf("%s", err), http.StatusInternalServerError)
			return
		}

		if total != 0 {

			if repoType != GITLAB {
				for _, reposUser := range gogsGitHubResp { //遍历map
					for index, repo := range reposUser.Repos { //遍历仓库数组

						for _, project := range mamageProjectList {
							id, _ := strconv.Atoi(project.GitlabProjectId)
							if id == repo.ProjectId {
								reposUser.Repos[index].ManagedProject.Id = project.Id
								reposUser.Repos[index].ManagedProject.Active = 1
							}

						}

					}
				}
			} else {
				for index, repo := range repoListGitLab {
					for _, project := range mamageProjectList {
						id, _ := strconv.Atoi(project.GitlabProjectId)
						if id == repo.ProjectId {
							repoListGitLab[index].ManagedProject.Id = project.Id
							repoListGitLab[index].ManagedProject.Active = 1
						}

					}
				}

			}
		}

	} else if repoType == SVN {
		glog.V(1).Info("%s %s", method, "Do not support repository list for SVN")

	} else {
		glog.Errorf(" %s not support repo type repoType: %s\n", method, repoType)
		cirepo.ResponseErrorAndCode("Only support gitlab/github/svn/gogs for now", http.StatusBadRequest)
		return
	}

	if repoType == GOGS || repoType == GITHUB {
		cirepo.ResponseResultAndStatusDevops(gogsGitHubResp, http.StatusOK)
		return
	}

	if repoType == GITLAB {
		cirepo.ResponseResultAndStatusDevops(repoListGitLab, http.StatusOK)
		return
	}

	cirepo.ResponseResultAndStatusDevops("Do not support repository list for SVN", http.StatusOK)
	return

}

// @router /:type [DELETE]
func (cirepo *CiReposController) Logout() {

	method := "CiReposController.Logout"

	repoType := cirepo.Ctx.Input.Param(":type")

	cirepo.Audit.SetResourceName(repoType)
	cirepo.Audit.SetOperationType(models.AuditOperationDelete)
	cirepo.Audit.SetResourceType(models.AuditResourceRepos)

	if cirepo.NamespaceType == TeamSpace && cirepo.User.Role == 0 {

		cirepo.ResponseErrorAndCode("必须是系统管理员或者团队管理员才能注销代码仓库 ", http.StatusUnauthorized)
		return
	}

	depot := models.NewCiRepos()
	_, err := depot.DeleteOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType))

	if err != nil {
		parseResult, _ := sqlstatus.ParseErrorCode(err)
		if parseResult == sqlstatus.SQLErrNoRowFound {
			glog.Errorf(" %s the repo info not exist: %v\n", method, err)
			cirepo.ResponseErrorAndCode("the repo info not exist", http.StatusNotFound)
			return
		} else {
			glog.Errorf(" %s delete repo info from database failed: %v\n", method, err)
			cirepo.ResponseErrorAndCode("delete repo info from database failed ", http.StatusInternalServerError)
			return
		}
	}

	cirepo.ResponseSuccessStatusAndMessageDevops("Logout successfully")
	return

}

type AddRepositoryReq struct {
	Url               string `json:"url"`
	PrivateToken      string `json:"private_token"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	AccessTokenSecret string `json:"access_token_secret"`
}

// @router /:type [POST]
func (cirepo *CiReposController) AddRepository() {

	method := "CiReposController.AddRepository"

	repoType := cirepo.Ctx.Input.Param(":type")
	cirepo.Audit.SetResourceName(repoType)
	cirepo.Audit.SetOperationType(models.AuditOperationCreate)
	cirepo.Audit.SetResourceType(models.AuditResourceRepos)
	reqBody := cirepo.Ctx.Input.RequestBody
	if string(reqBody) == "" {
		glog.Errorf("%s Request body is empty\n", method)
		cirepo.ResponseErrorAndCode("Request body is empty", http.StatusBadRequest)
		return
	}
	body := AddRepositoryReq{}

	err := json.Unmarshal(reqBody, &body)
	if err != nil {
		glog.Errorf(" %s json Unmarshal reqBody data failed===>: %v\n", method, err)
		cirepo.ResponseErrorAndCode("json Unmarshal reqBody data failed", http.StatusInternalServerError)
		return
	}

	if repoType == GITLAB || repoType == GOGS {

		if body.Url == "" || body.PrivateToken == "" {
			glog.Errorf(" %s repo url and private tocken is required \n", method)
			cirepo.ResponseErrorAndCode("repo url and private tocken is required", http.StatusBadRequest)
			return
		}

	} else if repoType == SVN {
		if body.Url == "" {

			glog.Errorf(" %s Address of SVN repository is required. \n", method)
			cirepo.ResponseErrorAndCode("Address of SVN repository is required.", http.StatusBadRequest)
			return
		}
		//属于私库时
		if body.Username != "" && body.Password == "" {
			glog.Errorf("%s Password of SVN user is required. \n", method)
			cirepo.ResponseErrorAndCode("Address of SVN repository is required.", http.StatusBadRequest)
			return
		}

	}

	depot := models.NewCiRepos()
	err = depot.FindOneRepoToken(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		res, _ := sqlstatus.ParseErrorCode(err)
		if res == sqlstatus.SQLErrNoRowFound {

		} else {
			glog.Errorf(" %s get repos tocken info from database failed: %v\n", method, err)
			cirepo.ResponseErrorAndCode("get repos tocken info from database failed ", http.StatusInternalServerError)
			return
		}
	}

	//var userInfos []coderepo.UserInfo
	////TODO, use the user/org later auth
	//if depot.UserInfo != "" {
	//	err = json.Unmarshal([]byte(depot.UserInfo), &userInfos)
	//	if err != nil {
	//		glog.Errorf(" %s json Unmarshal failed===>: %v\n", method, err)
	//		cirepo.ResponseErrorAndCode("json Unmarshal failed", 501)
	//		return
	//	}
	//	glog.V(1).Infof("User < %s > Is already authorized.", cirepo.Namespace)
	//	cirepo.ResponseMessageAndResultAndStatusDevops(userInfos, fmt.Sprintf("User < %s > Is already authorized.", cirepo.Namespace),
	//		http.StatusOK)
	//
	//	return
	//}
	
	//depo exist but user_info is empty will delete data from database
	if depot != nil {
		depot.DeleteOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType))
	}

	repoInfo := models.CiRepos{}
	repoInfo.UserId = int(cirepo.User.UserID)
	repoInfo.RepoType = models.DepotToRepoType(repoType)
	repoInfo.CreateTime = time.Now()
	repoInfo.IsEncrypt = 1
	repoInfo.Namespace = cirepo.Namespace
	results := ""
	if repoType == SVN {
		repoInfo.AccessUserName = body.Username
		repoInfo.AccessToken = body.Password
		repoInfo.GitlabUrl = body.Url
		results = "SVN repository was added successfully"

		_, err := depot.CreateOneRepo(repoInfo)
		if err != nil {
			results = "insert svn info into database failed "
			cirepo.ResponseErrorAndCode(results, http.StatusInternalServerError)
			return
		}
		cirepo.ResponseErrorAndCode(results, http.StatusOK)
		return
	}
	//githun gogs gitlab
	repoApi := coderepo.NewRepoApier(repoType, body.Url, body.PrivateToken)
	//TODO 暂时不支持github
	if repoType != GITLAB && repoType != GOGS {
		cirepo.ResponseErrorAndCode("not support this repo "+repoType, http.StatusUnauthorized)
		return
	}

	userInfo, err := repoApi.GetUserAndOrgs()
	if err != nil {
		glog.Errorf("%s get user and orgs failed:>%v", method, err)
		cirepo.ResponseErrorAndCode("get user and orgs failed", http.StatusInternalServerError)
		return
	}
	glog.Infof("userInfo=%v\n", userInfo)
	repoInfo.AccessUserName = userInfo[0].Login
	repoInfo.AccessToken = body.PrivateToken
	repoInfo.AccessTokenSecret = body.AccessTokenSecret
	repoInfo.GitlabUrl = body.Url
	userRepoInfos, err := json.Marshal(userInfo)
	if err != nil {
		glog.Errorf("%s json marshal userinfo  failed:>%v", method, err)
		cirepo.ResponseErrorAndCode("userinfo json marshal failed", http.StatusInternalServerError)
		return
	}
	repoInfo.UserInfo = string(userRepoInfos)

	_, err = depot.CreateOneRepo(repoInfo)
	if err != nil {
		results = "insert gitlab or gogs info into database failed "
		cirepo.ResponseErrorAndCode(results, http.StatusInternalServerError)
		return
	}

	if repoType != SVN {
		repositrys, err := repoApi.GetAllUsersRepos(userInfo)
		if err != nil {
			glog.Errorf(" %s get all user repos failed: %v\n", method, err)
			cirepo.ResponseErrorAndCode(" 同步代码仓库失败:"+fmt.Sprintf("%s", err), http.StatusInternalServerError)
			return
		}
		var repoList string

		if repoType == GOGS || repoType == GITHUB {
			var repoInfo = make(map[string][]coderepo.ReposGitHubAndGogs)
			repoInfo[repositrys[0].Owner.Name] = repositrys
			data, err := json.Marshal(repoInfo)
			if err != nil {
				glog.Errorf("%s github or gogs json marshal failed:%v\n", method, err)
				cirepo.ResponseErrorAndCode("github or gogs  同步代码仓库失败 json marshal failed: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
				return
			}
			repoList = string(data)
		} else if repoType == GITLAB {
			data, err := json.Marshal(repositrys)
			if err != nil {
				glog.Errorf("%s gitlab json marshal failed:%v\n", method, err)
				cirepo.ResponseErrorAndCode("gitlab 同步代码仓库失败 json marshal failed: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
				return
			}
			repoList = string(data)
		}

		//update database repolist
		resultUpdate, err := depot.UpdateOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType), repoList)
		if resultUpdate < 1 || err != nil {

			glog.Errorf("%s update repo_list failed from database\n", method)

			cirepo.ResponseErrorAndCode(" 同步代码仓库失败 repo list not update: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)

			return
		}
	}

	cirepo.ResponseResultAndStatusDevops(userInfo, http.StatusOK)
	return

}

// @router /:type [PUT]
func (cirepo *CiReposController) SyncRepos() {
	method := "controllers/CiReposController.SyncRepos"
	repoType := cirepo.Ctx.Input.Param(":type")
	cirepo.Audit.SetResourceName(repoType)
	cirepo.Audit.SetOperationType(models.AuditOperationUpdate)
	cirepo.Audit.SetResourceType(models.AuditResourceRepos)
	depot := models.NewCiRepos()
	err := depot.FindOneRepoToken(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		glog.Errorf(" %s get repos tocken info from database failed: %v\n", method, err)
		cirepo.ResponseErrorAndCode("No repo tocken info found.", http.StatusNotFound)
		return
	}

	var userInfos []coderepo.UserInfo

	//TODO, use the user/org later
	if depot.UserInfo != "" {
		err = json.Unmarshal([]byte(depot.UserInfo), &userInfos)
		if err != nil {
			glog.Errorf(" %s json Unmarshal user info: %v\n", method, err)
			cirepo.ResponseErrorAndCode("json Unmarshal user info failed", http.StatusInternalServerError)
			return
		}
	}
	//同步
	repoApi := coderepo.NewRepoApier(repoType, depot.GitlabUrl, depot.AccessToken)
	if repoApi == nil {
		glog.Errorf(" %s 暂时不支持这个类型 : %v\n", method, err)
		cirepo.ResponseErrorAndCode("暂时不支持这个类型", http.StatusInternalServerError)
		return
	}

	repositrys, err := repoApi.GetAllUsersRepos(userInfos)
	if err != nil {
		glog.Errorf(" %s get all user repos failed===>: %v\n", method, err)
		cirepo.ResponseErrorAndCode(" 同步代码仓库失败:"+fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}

	if len(repositrys) == 0 {
		glog.Errorf("%s get all user repos failed===>: %v\n", method, err)
		cirepo.ResponseErrorAndCode(" 同步代码仓库失败:"+fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}

	var repoList string

	if repoType == GOGS || repoType == GITHUB {
		var repoInfo = make(map[string][]coderepo.ReposGitHubAndGogs)
		repoInfo[repositrys[0].Owner.Name] = repositrys
		data, err := json.Marshal(repoInfo)
		if err != nil {
			glog.Errorf("%s github or gogs json marshal failed:%v\n", method, err)
			cirepo.ResponseErrorAndCode("github or gogs  同步代码仓库失败 json marshal failed: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
			return
		}
		repoList = string(data)
	} else if repoType == GITLAB {
		data, err := json.Marshal(repositrys)
		if err != nil {
			glog.Errorf("%s gitlab json marshal failed:%v\n", method, err)
			cirepo.ResponseErrorAndCode("gitlab 同步代码仓库失败 json marshal failed: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)
			return
		}
		repoList = string(data)
	}

	//update database repolist
	resultUpdate, err := depot.UpdateOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType), repoList)
	if err != nil {

		glog.Errorf("%s update repo_list failed from database resultUpdate=%d err=%v\n", method, resultUpdate, err)

		cirepo.ResponseErrorAndCode(" 同步代码仓库失败 repo list not update: "+fmt.Sprintf("%s", err), http.StatusInternalServerError)

		return
	}

	//gogs gihub resp
	var gogsGitHubResp = make(map[string]GogsGithubRepoResp)
	//glog.Infof("%s depot.RepoList=%v",method,depot.RepoList)
	var repoListGitLab []coderepo.ReposGitHubAndGogs
	if repoType == GOGS || repoType == GITLAB || repoType == GITHUB {
		if repoType != GITLAB {

			var reposUser GogsGithubRepoResp

			reposUser.User = repositrys[0].Owner.Name
			reposUser.Repos = repositrys
			gogsGitHubResp[repositrys[0].Owner.Name] = reposUser

		} else {
			repoListGitLab = repositrys
		}

		//显示激活
		mamageProjectList, total, err := models.NewCiManagedProjects().ListProjectsByType(cirepo.Namespace, repoType)
		if err != nil {
			glog.Errorf(" %s get manageProject list %s failed: %v\n", method, cirepo.Namespace, err)
			cirepo.ResponseErrorAndCode(" get manageProject list namespace=["+cirepo.Namespace+"] failed:"+fmt.Sprintf("%s", err), http.StatusInternalServerError)
			return
		}

		if total != 0 {

			if repoType != GITLAB {
				for _, reposUser := range gogsGitHubResp { //遍历map
					for index, repo := range reposUser.Repos { //遍历仓库数组

						for _, project := range mamageProjectList {
							id, _ := strconv.Atoi(project.GitlabProjectId)
							if id == repo.ProjectId {
								reposUser.Repos[index].ManagedProject.Id = project.Id
								reposUser.Repos[index].ManagedProject.Active = 1
							}

						}

					}
				}
			} else {
				for index, repo := range repoListGitLab {
					for _, project := range mamageProjectList {
						id, _ := strconv.Atoi(project.GitlabProjectId)
						if id == repo.ProjectId {
							repoListGitLab[index].ManagedProject.Id = project.Id
							repoListGitLab[index].ManagedProject.Active = 1
						}

					}
				}

			}
		}

	} else {
		glog.Errorf(" %s not support repo type repoType: %s\n", method, repoType)
		cirepo.ResponseErrorAndCode("Only support gitlab/github/svn/gogs for now", http.StatusBadRequest)
		return
	}

	if repoType == GOGS || repoType == GITHUB {
		cirepo.ResponseResultAndStatusDevops(gogsGitHubResp, http.StatusOK)
		return
	}

	if repoType == GITLAB {
		cirepo.ResponseResultAndStatusDevops(repoListGitLab, http.StatusOK)
		return
	}

	cirepo.ResponseResultAndStatusDevops("Do not support repository list for SVN", http.StatusOK)
	return

}

// @router /supported [get]
func (cirepo *CiReposController) GetSupportedRepos() {

	supportedRepos := []string{"github", "gitlab", "svn", "gogs"}

	CLIENT_ID := os.Getenv("CLIENT_ID")
	CLIENT_SECRET := os.Getenv("CLIENT_SECRET")
	GITHUB_REDIRECT_URL := os.Getenv("GITHUB_REDIRECT_URL")

	if CLIENT_ID == "" || CLIENT_SECRET == "" || GITHUB_REDIRECT_URL == "" {

		data, err := json.Marshal(supportedRepos[1:4])
		if err != nil {
			glog.Errorf("json marshal failed:%v\n", err)
		}
		cirepo.Ctx.ResponseWriter.Write(data)
		cirepo.Ctx.ResponseWriter.Status = http.StatusOK
		return
	}

	data, err := json.Marshal(supportedRepos)
	if err != nil {
		glog.Errorf("json marshal failed:%v\n", err)
	}
	cirepo.Ctx.ResponseWriter.Write(data)
	cirepo.Ctx.ResponseWriter.Status = http.StatusOK
	return
}

// @router /:type/auth [get]
func (cirepo *CiReposController) GetAuthRedirectUrl() {

	repoType := cirepo.Ctx.Input.Param(":type")

	if repoType != GITHUB {

		cirepo.ResponseErrorAndCode("Only support to get redirect url of github.", 400)
		return
	}

	CLIENT_ID := os.Getenv("CLIENT_ID")
	CLIENT_SECRET := os.Getenv("CLIENT_SECRET")
	GITHUB_REDIRECT_URL := os.Getenv("GITHUB_REDIRECT_URL")

	if CLIENT_ID == "" || CLIENT_SECRET == "" || GITHUB_REDIRECT_URL == "" {

		cirepo.ResponseErrorAndCode("client_id i empty or client_secret is empty "+
			"or github_redirest_url is empty", 400)
		return
	}

	authRedirectUrl := fmt.Sprintf("%s/authorize?client_id=%s&redirect_uri=%s&"+
		"redirect_uri=%s&state=%s&scope=repo,user:email", coderepo.GitHubLoginUrl, CLIENT_ID, GITHUB_REDIRECT_URL,
		uuid.NewV4().String())

	cirepo.ResponseSuccess(authRedirectUrl)

	return

}

type UserInfo struct {
	Avatar string `json:"avatar"`
	Email  string `json:"email"`
	Id     int  `json:"id"`
	Login  string `json:"login"`
	Type   string `json:"type"`
	Url    string `json:"url"`
}

// @router /:type/user [get]
func (cirepo *CiReposController) GetUserInfo() {
	method := "CiReposController.GetUserInfo"
	repoType := cirepo.Ctx.Input.Param(":type")

	depot := models.NewCiRepos()
	err := depot.FindOneRepo(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		glog.Errorf(" %s get repos auth info from database failed: %v\n", method, err)
		cirepo.ResponseErrorAndCode("No repo auth info found.", http.StatusNotFound)
		return
	}

	var username string
	var userInfos []UserInfo
	//TODO, use the user/org later
	if depot.UserInfo != "" {
		err = json.Unmarshal([]byte(depot.UserInfo), &userInfos)
		if err != nil {
			glog.Errorf(" %s json Unmarshal user info failed===>: %v\n", method, err)
			cirepo.ResponseErrorAndCode("json Unmarshal user info failed", http.StatusInternalServerError)
			return
		}

		username = userInfos[0].Login

	} else {
		username = depot.AccessUserName
	}

	response := struct {
		Username string `json:"username"`
		Url      string `json:"url"`
		Depo     string `json:"depo"`
		//models.CiRepos
	}{
		Username: username,
		Url:      depot.GitlabUrl,
		Depo:     repoType,
		//CiRepos:  *depot,
	}

	cirepo.ResponseResultAndStatusDevops(response, http.StatusOK)

	return

}

// @router /:type/tags [GET]
func (cirepo *CiReposController) GetTags() {

	method := "CiReposController.GetTags"
	repoType := cirepo.Ctx.Input.Param(":type")
	repoName := cirepo.GetString("reponame")
	projectId, _ := cirepo.GetInt("project_id")
	if repoType != GITLAB {
		if repoName == "" || strings.Index(repoName, "/") < 0 {
			glog.Errorf("reponame in query is required and must be fullname. \n")
			cirepo.ResponseErrorAndCode("reponame in query is required and must be fullname.", http.StatusBadRequest)
			return
		}
	}

	if repoType == GITLAB && projectId == 0 {
		glog.Errorf("project_id in query is required. \n")
		cirepo.ResponseErrorAndCode("project_id in query is required.", http.StatusBadRequest)
		return
	}

	depot := models.NewCiRepos()
	// If the user already logout
	err := depot.FindOneRepoToken(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		glog.Errorf(" %s Not authorized to access branch information, make sure you already logged in.: %v\n", method, err)
		cirepo.ResponseErrorAndCode("Not authorized to access branch information, make sure you already logged in.", http.StatusUnauthorized)
		return
	}

	repoApi := coderepo.NewRepoApier(repoType, depot.GitlabUrl, depot.AccessToken)
	//gogs 还没有支持获取tag的接口
	if repoType != GOGS {
		tags, err := repoApi.GetRepoAllTags(repoName, projectId)
		if err != nil {
			glog.Errorf("%s get repo tags failed from %s server ==> %v\n", method, repoType, err)
			cirepo.ResponseErrorAndCode("get repo tags failed from "+repoType+" server ", http.StatusInternalServerError)
			return
		}

		glog.V(1).Infof("%s the tags info: %v", method, tags)

		cirepo.ResponseResultAndStatusDevops(tags, http.StatusOK)
		return
	} else {

		tags, err := repoApi.GetRepoAllTags(repoName, projectId)
		if err != nil {
			glog.Errorf("%s get repo tags failed from %s server ==> %v\n", method, repoType, err)
			//cirepo.ResponseErrorAndCode("get repo tags failed from "+repoType+" server ", 500)
			//return
		}
		glog.V(1).Infof("%s the tags info: %v", method, tags)

		cirepo.ResponseResultAndStatusDevops(tags, http.StatusOK)

		return
	}

	cirepo.ResponseResultAndStatusDevops("", http.StatusOK)
	return

}

// @router /:type/branches [GET]
func (cirepo *CiReposController) GetBranches() {

	method := "CiReposController.GetBranches"
	repoType := cirepo.Ctx.Input.Param(":type")
	repoName := cirepo.GetString("reponame")
	projectId, _ := cirepo.GetInt("project_id")

	if repoName == "" || strings.Index(repoName, "/") < 0 {
		glog.Errorf("reponame in query is required and must be fullname. reponame in query is required. \n")
		cirepo.ResponseErrorAndCode("reponame in query is required and must be fullname. reponame in query is required.", 400)
		return
	}
	glog.Infof("repoName=%s\n", repoName)
	if repoType == GITLAB && projectId == 0 {
		glog.Errorf("project_id in query is required. \n")
		cirepo.ResponseErrorAndCode("project_id in query is required.", 400)
		return
	}

	depot := models.NewCiRepos()
	// If the user already logout
	err := depot.FindOneRepoToken(cirepo.Namespace, models.DepotToRepoType(repoType))
	if err != nil {
		glog.Errorf(" %s Not authorized to access branch information, make sure you already logged in.: %v\n", method, err)
		cirepo.ResponseErrorAndCode("Not authorized to access branch information, make sure you already logged in.", 401)
		return
	}

	repoApi := coderepo.NewRepoApier(repoType, depot.GitlabUrl, depot.AccessToken)
	branchs, err := repoApi.GetRepoAllBranches(repoName, projectId)
	if err != nil {
		glog.Errorf("%s get repo branchs failed from %s server ==> %v\n", method, repoType, err)
		cirepo.ResponseErrorAndCode("get repo branchs failed from "+repoType+" server ", 500)
		return
	}

	glog.V(1).Infof("%s projectId=%d,repoType=%s,  the branchs  info: %v", method, projectId, repoType, branchs)

	cirepo.ResponseSuccessCIRuleDevops(branchs)

	return

}
