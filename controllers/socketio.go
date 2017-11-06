package controllers

import (
	"github.com/golang/glog"
	//"github.com/googollee/go-engine.io"
	//"github.com/googollee/go-engine.io/websocket"
	//"github.com/googollee/go-engine.io/transport"
	"github.com/googollee/go-socket.io"
	"encoding/json"
	"dev-flows-api-golang/models"
	"fmt"
	"dev-flows-api-golang/models/common"
	"k8s.io/client-go/pkg/api/v1"
	"text/template"
	"io"
)

var StageBuildLog, StageBuildStatus *socketio.Server

func init() {
	NewStageBuildSocket()
}

func NewStageBuildSocket() {
	var trans =[]string{"websocket"}

	var err error
	method := "NewStageBuildSocket"
	StageBuildLog, err = socketio.NewServer(trans)
	if err != nil {
		glog.Errorf("%s New StageBuildLog failed:==>[%s]\n", method, err)
		panic(err)
		return
	}

	go BuildLogSocketio()

	StageBuildStatus, err = socketio.NewServer(nil)
	if err != nil {
		glog.Errorf("%s new StageBuildStatus failed:==>[%s]\n", method, err)
		panic(err)
		return
	}
	go BuildStatusSocketio()
}

func BuildLogSocketio() {
	method:="BuildLogSocketio"
	StageBuildLog.On("connection", func(so socketio.Socket) {
		glog.Infof("%s connect user build log socket id is: %s\n", method,so.Id())
		SocketController(so)
	})

}


func BuildStatusSocketio() {
	method:="BuildStatusSocketio"
	StageBuildStatus.On("connection", func(so socketio.Socket) {
		glog.Infof("%s connect user build status socket id is: %s\n",method, so.Id())
		JobWatcher(so)
	})

}
//判断stage构建状态，如果已为失败或构建完成，则从ElasticSearch中获取日志
//如构建中，则从k8s API获取实时日志
type BuildMessage struct {
	FlowId        string `json:"flowId"`
	FlowBuildId   string `json:"flowBuildId"`
	StageBuildId  string `json:"stageBuildId"`
	StageId       string `json:"stageId"`
	ContainerName string `json:"containerName"`
	PodName       string `json:"podName"`
	JobName       string `json:"jobName"`
	NodeName      string `json:"nodeName"`
	Status        int    `json:"status"`
	ControllerUid string `json:"controller_id"`
	LogData       string `json:"logData"`
	ClusterId     string `json:"cluster_id"`
}

const CILOG = "ciLogs"
const TailLines = 200
const POD_INIT = "pod-init"
const GET_LOG_RETRY_COUNT = 3
const GET_LOG_RETRY_MAX_INTERVAL = 30

var SocketLogRespData = make(chan interface{}, 4096)

//判断stage构建状态，如果已为失败或构建完成，则从ElasticSearch中获取日志
//如构建中，则从k8s API获取实时日志
//SocketController
func SocketController(socket socketio.Socket) {
	method := "SocketController"
	defer socket.Disconnect()
	go func() {
		var buildMessage BuildMessage
		socket.On("ciLogs", func(msg string) {

			err := json.Unmarshal([]byte(msg), &buildMessage)
			if err != nil {
				glog.Errorf("%s json unmarshal failed====>\n", method, err)
				socket.Emit(CILOG, `<font color="red">[Enn Flow API Error] Missing parameters.</font>\n`)
				socket.Disconnect()
				return
			}
			if message := CheckLogData(buildMessage); message != "" {
				glog.Errorf("%s Missing parameters====>\n", method, err)
				glog.Errorf("Missing parameters====>>:[%v]\n", buildMessage)
				socket.Emit(CILOG, message)
				socket.Disconnect()
				return
			}

			GetStageBuildLogsFromK8S(buildMessage, socket)
			//TODO 结束的时候的问题
			//socket.emit('ciLogs-ended', 'Failed to get logs')
			//logger.error(method, 'Failed to get logs:', err)
			//socket.disconnect()

		})

		socket.On("disconnect", func() {
			glog.Infof("==============>>user disconnected<<===========\n")
			socket.Emit("user disconnected")
			socket.Disconnect()
			return

		})
		socket.On("error", func(err error) {
			glog.Errorf(" socket error:%v\n", err)
			socket.Emit("error", err)
			socket.Disconnect()
			return
		})
	}()

}

//GetStageBuildLogsFromK8S
func GetStageBuildLogsFromK8S(buildMessage BuildMessage, socket socketio.Socket) {

	method := "GetStageBuildLogsFromK8S"

	imageBuilder := models.NewImageBuilder()

	build, err := GetValidStageBuild(buildMessage.FlowId,buildMessage.StageId,buildMessage.StageBuildId)
	if err != nil {
		glog.Errorf("%s GetValidStageBuild failed===>%v\n", method, err)
		socket.Emit(CILOG, err)
		return
	}

	if build.Status == common.STATUS_WAITING {
		buildStatus := struct {
			BuildStatus string `json:"buildStatus"`
		}{
			BuildStatus: "waiting",
		}
		glog.Infof("%s the stage is onwaiting ===>%v\n", method, build)
		socket.Emit(CILOG, buildStatus)
		return
	}
	if build.PodName == "" {
		podName, err := imageBuilder.GetPodName(build.Namespace, build.JobName)
		if err != nil || podName == "" {
			glog.Errorf("%s get job name=[%s] pod name failed:======>%v\n", method, build.JobName, err)
			socket.Emit(CILOG, err)
			return
		}
		models.NewCiStageBuildLogs().UpdatePodNameById(podName, build.BuildId)
		GetLogsFromK8S(imageBuilder, build.Namespace, build.JobName, build.PodName, socket)
		return
	}

	GetLogsFromK8S(imageBuilder, build.Namespace, build.JobName, build.PodName, socket)
	return

}

//GetLogsFromK8S
func GetLogsFromK8S(imageBuilder *models.ImageBuilder, namespace, jobName, podName string, socket socketio.Socket) {
	imageBuilder.WatchEvent(namespace, podName, socket)
	WaitForLogs(imageBuilder, namespace, podName, models.SCM_CONTAINER_NAME, socket)
	WaitForLogs(imageBuilder, namespace, podName, models.BUILDER_CONTAINER_NAME, socket)


}

func Int64Toint64Point(input int64) *int64 {
	tmp := new(int64)
	*tmp = int64(input)
	return tmp

}

//WaitForLogs websocket get logs
func WaitForLogs(imageBuild *models.ImageBuilder, namespace, podName, containerName string, socket socketio.Socket) {
	method := "WaitForLogs"
	follow := false
	previous := true
	if socket != nil {
		follow = true
		previous = false
	}
	opt := &v1.PodLogOptions{
		Container:  containerName,
		TailLines:  Int64Toint64Point(TailLines),
		Previous:   previous, //
		Follow:     follow,
		Timestamps: true,
	}
	//相应websocket的请求
	if socket != nil {
		readCloser, err := imageBuild.Client.Pods(namespace).GetLogs(podName, opt).Stream()
		if err != nil {
			glog.Errorf("%s socket get pods log readCloser faile from kubernetes:==>%v\n", method, err)
			socket.Emit("ciLogs-ended", fmt.Sprintf(`<font color="red">[Enn Flow API Error] Failed to get log of %s</font>\n`, podName))
			return
		}

		if containerName == models.BUILDER_CONTAINER_NAME {
			socket.On("stop_receive_log", func() {
				glog.Infof("==============>> user stop_receive_log user <<===========\n")
				socket.Emit(CILOG, fmt.Sprintf(`<font color="#ffc20e">[Enn Flow API] 您停止了接收日志</font>\n`))
				return
			})
			socket.Emit(CILOG, "---------------------------------------------------")
			socket.Emit(CILOG, "--- 子任务容器: 仅显示最近 "+fmt.Sprintf("%d", TailLines)+" 条日志 ---")
			socket.Emit(CILOG, "---------------------------------------------------")
		}
		data := make([]byte, 1024*1024, 1024*1024)
		for {
			n, err := readCloser.Read(data)
			if nil != err {
				if err == io.EOF {
					glog.Infof("%s [Enn Flow API ] finish get log of %s.%s!\n", method, podName, containerName)
					glog.Infof("==========>>Get log successfully from socket.!!<<============\n")
					socket.Emit("ciLogs-ended", fmt.Sprintf(`<font color="red">[Enn Flow API ] 日志读取结束  %s.%s!</font>\n`, podName, containerName))
					return
				}
				return
			}

			logMessage := &LogMessage{
				Name: containerName,
				Log:  template.HTMLEscapeString(string(data[:n])),
			}
			message, err := json.Marshal(logMessage)
			if nil != err {
				glog.Warningf("%s [Enn Flow API Error] Parse container log failed, container name is %s.%s Error:==>%v\n", method, podName, containerName, err)
				socket.Emit(CILOG, fmt.Sprintf(`<font color="red">[Enn Flow API Error] 日志读取失败,请重试  %s.%s!</font>\n`, podName, containerName))
				return
			}

			socket.Emit(CILOG, message)

		}
		//相应ES获取日志的请求
	} else {
		glog.Errorf("the socket is nil\n")
	}
	return

}

func GetValidStageBuild(flowId, stageId, stageBuildId string) (models.CiStageBuildLogs, error) {
	var build models.CiStageBuildLogs
	method := "SocketLogController.GetValidStageBuild"
	stage, err := models.NewCiStage().FindOneById(stageId)
	if err != nil {
		glog.Errorf("%s find stage by stageId failed or not exist from database: %v\n", method, err)
		return build, err
	}
	if flowId != stage.FlowId {

		return build, fmt.Errorf("Stage is not %s in the flow\n", stageId)
	}

	build, err = models.NewCiStageBuildLogs().FindOneById(stageBuildId)
	if err != nil {
		glog.Errorf("%s find stagebuild by StageBuildId failed or not exist from database: %v\n", method, err)
		return build, err
	}

	if stage.StageId != build.StageId {

		return build, fmt.Errorf("Build is not %s one of the stage \n", build.BuildId)

	}

	return build, nil
}
