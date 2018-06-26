package models

import (
	"os"
	"dev-flows-api-golang/modules/client"
	"github.com/astaxie/beego/context"
	"github.com/golang/glog"
	"io"
	//"text/template"
	"fmt"
	"time"
	//"encoding/json"
	v1beta1 "k8s.io/client-go/pkg/apis/batch/v1"
	"github.com/googollee/go-socket.io"
	"text/template"
	//"dev-flows-api-golang/util/rand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	//"k8s.io/client-go/pkg/api"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/fields"
	"strings"
	"dev-flows-api-golang/models/common"
	"encoding/json"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SCM_CONTAINER_NAME         = "enn-scm"
	BUILDER_CONTAINER_NAME     = "enn-builder"
	BUILDER_SONAR_NAME         = "enn-sonar"
	DEPENDENT_CONTAINER_NAME   = "enn-deps"
	KIND_ERROR                 = "Status"
	MANUAL_STOP_LABEL          = "enn-manual-stop-flag"
	GET_LOG_RETRY_COUNT        = 3
	GET_LOG_RETRY_MAX_INTERVAL = 30
	BUILD_AT_SAME_NODE         = true
)

var (
	DEFAULT_IMAGE_BUILDER string
)

func init() {
	DEFAULT_IMAGE_BUILDER = os.Getenv("DEFAULT_IMAGE_BUILDER")
	if DEFAULT_IMAGE_BUILDER == "" {
		DEFAULT_IMAGE_BUILDER = "enncloud/image-builder:v2.2"
	}

}

type ImageBuilder struct {
	APIVersion  string
	Kind        string
	BuilderName string
	ScmName     string
	Client      *client.ClientSet
}

func NewImageBuilder(clusterID ...string) *ImageBuilder {
	var Client *client.ClientSet
	if len(clusterID) != 1 {
		client.Initk8sClient()
		Client = client.KubernetesClientSet
	} else {
		Client = client.GetK8sConnection(clusterID[0])
	}

	glog.Infof("get kubernetes info :%v\n", Client)
	return &ImageBuilder{
		BuilderName: BUILDER_CONTAINER_NAME,
		ScmName:     SCM_CONTAINER_NAME,
		Client:      Client,
		APIVersion:  "batch/v1",
		Kind:        "Job",
	}

}

func (builder *ImageBuilder) BuildImage(buildInfo BuildInfo, volumeMapping []Setting, registryConf string) (*v1beta1.Job, error) {
	method := "models/buildImage"
	jobTemplate := &v1beta1.Job{}
	jobTemplate.Spec.Template.Spec.RestartPolicy = apiv1.RestartPolicyNever
	//设置labels，方便通过label查询 ClusterID
	jobTemplate.ObjectMeta.Labels = builder.setBuildLabels(buildInfo)
	jobTemplate.Spec.Template.ObjectMeta.Labels = builder.setBuildLabels(buildInfo)
	//构造build container 获取构建镜像的镜像
	buildImage := ""
	if buildInfo.Build_image != "" {
		buildImage = buildInfo.Build_image
	} else {
		buildImage = DEFAULT_IMAGE_BUILDER
	}

	//从环境变量获取sonar镜像
	sonarImage := os.Getenv("SONAR_SCANNER")
	if sonarImage == "" {
		sonarImage = "qinzhao-harbor/sonar-scanner:v2.2"
	}

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      "repo-path",
			MountPath: buildInfo.Clone_location, //拉取代码到本地/app目录下
		},
		//// TODO: To see if we should remove this later
		{
			Name:      "localtime",
			MountPath: "/etc/localtime", //拉取代码到本地/app目录下
		},
	}
	// If it's to build image, force the command to EnnCloud image builder 构建镜像
	if buildInfo.Type == 3 {
		volumeMounts = []apiv1.VolumeMount{
			{
				Name:      "repo-path",
				MountPath: buildInfo.Clone_location, //拉取代码到本地/app目录下
			},
			//// TODO: To see if we should remove this later
			{
				Name:      "localtime",
				MountPath: "/etc/localtime",
			},
			{
				Name:      "docker-socket",
				MountPath: "/var/run/docker.sock",
			},
			{
				Name:      "registrysecret",
				MountPath: "/docker/secret",
				ReadOnly:  true,
			},
		}
	}
	// Bind to specified node selector
	BIND_BUILD_NODE := os.Getenv("BIND_BUILD_NODE")
	if BIND_BUILD_NODE == "" {
		BIND_BUILD_NODE = "true"
	}
	if BIND_BUILD_NODE == "true" {
		jobTemplate.Spec.Template.Spec.NodeSelector = map[string]string{
			"system/build-node": "true",
		}
	}

	// TODO: maybe update later
	if len(strings.Split(buildImage, "/")) == 2 {
		buildImage = common.HarborServerUrl + "/" + buildImage
	}
	//sonar
	if len(strings.Split(sonarImage, "/")) == 2 {
		sonarImage = common.HarborServerUrl + "/" + sonarImage
	}

	//指定到相同的节点上做CICD
	if BUILD_AT_SAME_NODE && buildInfo.NodeName != "" {
		//设置nodeName使得构建pod运行在同一node上
		jobTemplate.Spec.Template.Spec.NodeName = buildInfo.NodeName
	}
	//用户输入的构建的command 命令 并且不是构建镜像的
	jobTemplate.Spec.Template.Spec.Containers = make([]apiv1.Container, 0)
	jobContainer := apiv1.Container{
		Name:            BUILDER_CONTAINER_NAME,
		Image:           buildImage,
		ImagePullPolicy: apiv1.PullAlways,
		//Args:            buildInfo.Build_command,
		VolumeMounts: volumeMounts,
	}

	jobContainerSonar := apiv1.Container{
		Name:            BUILDER_SONAR_NAME,
		Image:           sonarImage,
		ImagePullPolicy: apiv1.PullAlways,
		VolumeMounts:    volumeMounts,
	}

	if len(buildInfo.Command) != 0 && buildInfo.Type != 3 {
		jobContainer.Command = buildInfo.Command
	} else if len(buildInfo.Command) == 0 && buildInfo.Type != 3 {
		jobContainer.Command = buildInfo.Build_command
	}

	if buildInfo.Type == 3 {
		jobContainer.WorkingDir = "/"
	} else {
		jobContainerSonar.WorkingDir = buildInfo.Clone_location
		jobContainer.WorkingDir = buildInfo.Clone_location
	}

	// If it's using online Dockerfile, no subfolder needed to specifiy
	if buildInfo.TargetImage.DockerfileOL != "" {
		glog.Infof("%s %s\n", method, "Using online Dockerfile, path will be default one")
		buildInfo.TargetImage.DockerfilePath = "/"
	}

	//构造dependency container 构建依赖

	if len(buildInfo.Dependencies) != 0 {
		for depIndex, dependencie := range buildInfo.Dependencies {
			dependencieContainer := apiv1.Container{
				Name:  DEPENDENT_CONTAINER_NAME + fmt.Sprintf("%s", depIndex),
				Image: common.HarborServerUrl + "/" + dependencie.Service,
			}
			if len(dependencie.Env) != 0 {
				dependencieContainer.Env = dependencie.Env
			}

			jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, dependencieContainer)

		}
	}

	jobTemplate.Spec.Template.Spec.Volumes = make([]apiv1.Volume, 0)
	jobTemplate.Spec.Template.Spec.HostNetwork = true
	volumes := []apiv1.Volume{
		{
			Name: "localtime",
			VolumeSource: apiv1.VolumeSource{
				HostPath: &apiv1.HostPathVolumeSource{
					Path: "/etc/localtime",
				},
			},
		},
		{
			Name: "repo-path",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
	}

	for _, volume := range volumes {
		jobTemplate.Spec.Template.Spec.Volumes = append(jobTemplate.Spec.Template.Spec.Volumes, volume)
	}

	if buildInfo.Type == 3 {
		volumesBuildImages := []apiv1.Volume{
			{
				Name: "docker-socket",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/var/run/docker.sock",
					},
				},
			},
			{
				Name: "registrysecret",
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "registrysecret",
					},
				},
			},
		}
		for _, volumesBuildImage := range volumesBuildImages {
			jobTemplate.Spec.Template.Spec.Volumes = append(jobTemplate.Spec.Template.Spec.Volumes, volumesBuildImage)
		}
	}
	//环境变量
	env := make([]apiv1.EnvVar, 0)
	if len(buildInfo.Env) != 0 {
		env = append(env, buildInfo.Env...)
	}
	glog.Infof("ENV==========%v\n", env)
	glog.Infof("buildInfo.Env==========%v\n", buildInfo.Env)
	// Used to build docker images burnabybull
	if buildInfo.Type == 3 {
		// Check the name of type of target image to build
		targetImage := buildInfo.TargetImage.Image
		targetImageTag := buildInfo.TargetImage.ImageTagType
		if targetImageTag == 1 {
			targetImage += ":" + buildInfo.Branch
		} else if targetImageTag == 2 {
			targetImage += ":" + time.Now().Format("20060102.150405.99")
		} else if targetImageTag == 3 {
			targetImage += ":" + buildInfo.TargetImage.CustomTag
		}

		registryUrl := common.HarborServerUrl
		//不支持第三方镜像库
		//if buildInfo.TargetImage.RegistryType==3{
		//	registryUrl=buildInfo.TargetImage.cus
		//}
		if buildInfo.TargetImage.DockerfileName != "" {
			env = append(env, apiv1.EnvVar{
				Name:  "DOCKERFILE_NAME",
				Value: buildInfo.TargetImage.DockerfileName,
			})
		}
		env = append(env, apiv1.EnvVar{
			Name:  "APP_CODE_REPO",
			Value: buildInfo.Clone_location,
		})

		env = append(env, apiv1.EnvVar{
			Name:  "IMAGE_NAME",
			Value: targetImage,
		})
		env = append(env, apiv1.EnvVar{
			Name:  "DOCKERFILE_PATH",
			Value: buildInfo.TargetImage.DockerfilePath,
		})
		env = append(env, apiv1.EnvVar{
			Name:  "REGISTRY",
			Value: registryUrl,
		})
	}

	// Handle stage link 这个type 分 source 和 target
	target := Setting{}
	for vindex, vMap := range volumeMapping {
		if "target" == vMap.Type {
			target = vMap
			target.Name = "volume-mapping-" + fmt.Sprintf("%d", vindex+1)
		}

		jobContainer.VolumeMounts = append(jobContainer.VolumeMounts, apiv1.VolumeMount{
			Name:      "volume-mapping-" + fmt.Sprintf("%d", vindex+1),
			MountPath: vMap.ContainerPath,
		})

		jobTemplate.Spec.Template.Spec.Volumes = append(jobTemplate.Spec.Template.Spec.Volumes, apiv1.Volume{
			Name: "volume-mapping-" + fmt.Sprintf("%d", vindex+1),
			VolumeSource: apiv1.VolumeSource{
				HostPath: &apiv1.HostPathVolumeSource{
					Path: vMap.VolumePath,
				},
			},
		})

	}

	jobTemplate.Spec.Template.Spec.Tolerations = []apiv1.Toleration{
		apiv1.Toleration{
			Key:      "system-jobType",
			Operator: apiv1.TolerationOpEqual,
			Value:    "devflows",
			Effect:   apiv1.TaintEffectNoSchedule,
		},
	}

	//构造init container
	jobTemplate.Spec.Template.Spec.InitContainers = make([]apiv1.Container, 0)
	initContainer := apiv1.Container{
		Name:            SCM_CONTAINER_NAME,
		Image:           buildInfo.ScmImage,
		ImagePullPolicy: "Always",
	}
	initContainer.Env = []apiv1.EnvVar{
		{
			Name:  "GIT_REPO",
			Value: buildInfo.RepoUrl,
		},
		{
			Name:  "GIT_TAG",
			Value: buildInfo.Branch,
		},
		{
			Name:  "GIT_REPO_URL",
			Value: buildInfo.Git_repo_url,
		},
		{
			Name:  "PUB_KEY",
			Value: buildInfo.PublicKey,
		},
		{
			Name:  "PRI_KEY",
			Value: buildInfo.PrivateKey,
		},
		{
			Name:  "REPO_TYPE",
			Value: buildInfo.RepoType,
		},
		{
			Name:  "DOCKERFILE_PATH",
			Value: buildInfo.TargetImage.DockerfilePath,
		},
		{
			Name:  "ONLINE_DOCKERFILE",
			Value: buildInfo.TargetImage.DockerfileOL,
		},
		{
			Name:  "SVN_USERNAME",
			Value: buildInfo.Svn_username,
		},
		{
			Name:  "SVN_PASSWORD",
			Value: buildInfo.Svn_password,
		},
		{
			Name:  "CLONE_LOCATION",
			Value: buildInfo.Clone_location,
		},
	}

	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "repo-path",
			MountPath: buildInfo.Clone_location,
		},
	}
	//sonar image  buildInfo.RepoUrl
	jobContainerSonar.Env = []apiv1.EnvVar{
		{
			Name:  "GIT_REPO",
			Value: SplitRepo(buildInfo.RepoUrl),
		},
		{
			Name:  "GIT_TAG",
			Value: buildInfo.Branch,
		},
		{
			Name:  "REPO_TYPE",
			Value: buildInfo.RepoType,
		},
	}

	//类型为构建，仓库类型为‘本地镜像仓库’

	if buildInfo.Type == 3 && buildInfo.TargetImage.RegistryType == 1 {
		initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
			Name:  "BUILD_DOCKER_IMAGE",
			Value: "1",
		}, apiv1.EnvVar{
			Name:  "IMAGE_NAME",
			Value: buildInfo.TargetImage.Image,
		}, apiv1.EnvVar{
			Name:  "FILES_PATH",
			Value: buildInfo.Clone_location + buildInfo.TargetImage.DockerfilePath,
		}, )

		if buildInfo.TargetImage.DockerfileName != "" {
			initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
				Name:  "DOCKERFILE_NAME",
				Value: buildInfo.TargetImage.DockerfileName,
			}, )
		}
	}

	if target.Name != "" {
		initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
			Name:  "PREVIOUS_BUILD_LEGACY_PATH",
			Value: target.ContainerPath,
		}, )
		initContainer.VolumeMounts = append(initContainer.VolumeMounts, apiv1.VolumeMount{
			Name:      target.Name,
			MountPath: target.ContainerPath,
		})
	}

	//用来标识是否是构建镜像,用来清除构建缓存
	if buildInfo.BUILD_INFO_TYPE == 1 && buildInfo.Type == 3 {
		initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
			Name:  "BUILD_INFO_TYPE",
			Value: "1",
		}, )
	} else if buildInfo.BUILD_INFO_TYPE == 2 && buildInfo.Type == 3 { //表示没有下一个stage了
		initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
			Name:  "BUILD_INFO_TYPE",
			Value: "2",
		}, )
	}

	for _, e := range buildInfo.Env {
		if e.Name == "SVNPROJECT" && "" != e.Value {
			initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
				Name:  "SVNPROJECT",
				Value: e.Value,
			}, )
			jobContainerSonar.Env = append(jobContainerSonar.Env, apiv1.EnvVar{
				Name:  "SVNPROJECT",
				Value: e.Value,
			}, )
		}

		if (e.Name == "SCRIPT_ENTRY_INFO" || e.Name == "SCRIPT_URL") && "" != e.Value && buildInfo.Type != 3 {
			initContainer.Env = append(initContainer.Env, apiv1.EnvVar{
				Name:  e.Name,
				Value: e.Value,
			}, )
		}
	}

	jobTemplate.ObjectMeta.GenerateName = builder.genJobName(buildInfo.FlowName, buildInfo.StageName)
	jobTemplate.ObjectMeta.Namespace = buildInfo.Namespace

	jobTemplate.Spec.Template.Spec.InitContainers = append(jobTemplate.Spec.Template.Spec.InitContainers, initContainer)
	dataInitContainerJob, _ := json.Marshal(jobTemplate.Spec.Template.Spec.InitContainers)
	jobTemplate.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"pod.alpha.kubernetes.io/init-containers": string(dataInitContainerJob),
	}

	jobContainer.Env = env

	jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, jobContainer)
	//不是构建镜像才把代码质量分析的镜像启动
	if buildInfo.Type != 3 {
		jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, jobContainerSonar)
	}

	return builder.Client.BatchV1Client.Jobs(buildInfo.Namespace).Create(jobTemplate)

}

type InitContainer struct {
	Name            string `json:"name"`
	Image           string `json:"image"`
	ImagePullPolicy string `json:"imagePullPolicy"`
	Envs            []apiv1.EnvVar `json:"env"`
	VolumeMounts    []apiv1.VolumeMount `json:"volumeMounts"`
}

func (builder *ImageBuilder) setBuildLabels(buildInfo BuildInfo) map[string]string {

	labels := map[string]string{
		"flow-id":        buildInfo.FlowName,
		"stage-id":       buildInfo.StageName,
		"stage-build-id": buildInfo.StageBuildId,
		"system/jobType": "devflows",
		"ClusterID":      buildInfo.ClusterID,
	}

	if buildInfo.FlowBuildId != "" {
		labels["flow-build-id"] = buildInfo.FlowBuildId
	}

	return labels

}
func (builder *ImageBuilder) genJobName(flowName, stageName string) string {

	return strings.Replace(strings.ToLower(flowName), "_", "-", -1) + "-" +
		strings.Replace(strings.ToLower(stageName), "_", "-", -1) + "-"

}

func (builder *ImageBuilder) GetLabel(flowName, stageName string) string {

	return strings.Replace(strings.ToLower(flowName), "_", "-", -1) + "-" +
		strings.Replace(strings.ToLower(stageName), "_", "-", -1) + "-"

}

func (builder *ImageBuilder) GetPodName(namespace, jobName string, buildId ...string) (string, error) {
	method := "GetPodName"
	if len(buildId) == 0 {
		pod, err := builder.GetPod(namespace, jobName)
		if err != nil {
			glog.Errorf("%s get pod name failed:====> %v\n", method, err)
			return "", err
		}

		return pod.ObjectMeta.Name, nil
	} else {
		pod, err := builder.GetPod(namespace, jobName, buildId[0])
		if err != nil {
			glog.Errorf("%s get pod name failed:====> %v\n", method, err)
			return "", err
		}

		return pod.ObjectMeta.Name, nil
	}

}

func (builder *ImageBuilder) GetPod(namespace, jobName string, stageBuildId ...string) (apiv1.Pod, error) {
	method := "ImageBuilder.GetPod"
	var podList apiv1.Pod
	labelsStr := ""
	glog.Infof("stageBuildId[0]======>>%v\n", stageBuildId)
	if len(stageBuildId) != 0 {
		labelsStr = fmt.Sprintf("stage-build-id=%s", stageBuildId[0])
	} else {
		labelsStr = fmt.Sprintf("job-name=%s", jobName)
	}

	labelsSel, err := labels.Parse(labelsStr)

	if err != nil {
		return podList, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labelsSel.String(),
	}
	pods, err := builder.Client.Pods(namespace).List(listOptions)
	if err != nil {
		glog.Errorf("%s get pod name failed:====> %v\n", method, err)
		return podList, err
	}

	if len(pods.Items) == 0 {
		return podList, fmt.Errorf("not found the pod")
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == apiv1.PodFailed {
			//优先获取失败状态的pod
			return pod, nil
		}

	}
	return pods.Items[0], nil

}

func (builder *ImageBuilder) GetPodByPodName(namespace, jobName, podName string, stageBuildId ...string) (apiv1.Pod, error) {
	method := "ImageBuilder.GetPod"
	var podList apiv1.Pod
	labelsStr := ""
	if len(stageBuildId) != 0 {
		labelsStr = fmt.Sprintf("stage-build-id=%s", stageBuildId[0])
	} else {
		labelsStr = fmt.Sprintf("job-name=%s", jobName)
	}

	labelsSel, err := labels.Parse(labelsStr)

	if err != nil {
		return podList, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labelsSel.String(),
	}
	pods, err := builder.Client.Pods(namespace).List(listOptions)
	if err != nil {
		glog.Errorf("%s get pod name failed:====> %v\n", method, err)
		return podList, err
	}

	if len(pods.Items) == 0 {
		return podList, fmt.Errorf("not found the pod")
	}

	for _, pod := range pods.Items {
		if pod.GetName() == podName {
			//优先获取失败状态的pod
			return pod, nil
		}

	}
	return pods.Items[0], nil

}

func (builder *ImageBuilder) WatchEvent(namespace, podName string, socket socketio.Socket) {
	if podName == "" {
		glog.Errorf("the podName is empty")
	}
	method := "WatchEvent"
	glog.Infoln("Begin watch kubernetes Event=====>>")
	fieldSelector, err := fields.ParseSelector(fmt.Sprintf("involvedObject.kind=pod,involvedObject.name=%s", podName))
	if nil != err {
		glog.Errorf("%s: Failed to parse field selector: %v\n", method, err)
		return
	}
	options := metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
		Watch:         true,
	}

	// 请求watch api监听pod发生的事件
	watchInterface, err := builder.Client.Events(namespace).Watch(options)
	if err != nil {
		glog.Errorf("get event watchinterface failed===>%v\n", method, err)
		socket.Emit("ciLogs", err)
		return
	}
	//TODO pod 不存在的情况
	for {
		select {
		case event, isOpen := <-watchInterface.ResultChan():
			if !isOpen {
				glog.Infof("%s the event watch the chan is closed\n", method)
				socket.Emit("ciLogs", "the event watch the chan is closed")
				break
			}
			glog.Infof("the pod event type=%s\n", event.Type)
			EventInfo, ok := event.Object.(*apiv1.Event)
			if ok {
				if strings.Index(EventInfo.Message, "PodInitializing") > 0 {
					socket.Emit("pod-init", builder.EventToLog(*EventInfo))
					continue
				}
				socket.Emit("ciLogs", builder.EventToLog(*EventInfo))
			}

		}

	}

	return

}

func (builder *ImageBuilder) EventToLog(event apiv1.Event) string {
	var color, level string
	if event.Type == "Normal" {
		color = "#5FB962"
		level = "Info"
	} else {
		color = "yellow"
		level = "Warn"
	}

	if level == "Warn" && event.Message != "" {
		if strings.Index(event.Message, "TeardownNetworkError:") > 0 {
			return ""
		}
	}

	return fmt.Sprintf(`<font color="%s">[%s] [%s]: %s</font>`, color, event.FirstTimestamp.Format(time.RFC3339), level, event.Message)
}

// 根据builder container的状态返回job状态 主要是获取容器的状态 scm container status
func TranslatePodStatus(status apiv1.PodStatus) {
	method := "TranslatePodStatus"
	//获取SCM 容器的状态
	if len(status.InitContainerStatuses) != 0 {
		for _, s := range status.InitContainerStatuses {
			if SCM_CONTAINER_NAME == s.Name {
				if s.State.Running != nil {
					glog.Infof("method=%s,Message=The scm container is still running [%s]\n", method, s.State.Running)
				}
				if s.State.Waiting != nil {
					glog.Infof("method=%s,Message=The scm container is still waiting [%s]\n", method, s.State.Waiting)
				}
				if s.State.Terminated != nil {
					if s.State.Terminated.ExitCode != 0 {
					}
					glog.Infof("method=%s,Message=The scm container is exit abnormally [%s]\n", method, s.State.Terminated)

				}
			}

		}
	}

	if len(status.ContainerStatuses) != 0 {
		for _, s := range status.ContainerStatuses {
			if BUILDER_CONTAINER_NAME == s.Name {
				if s.State.Running != nil {
					glog.Infof("method=%s,Message=The builder container is still running [%s]\n", method, s.State.Running)
					//statusMess.ContainerStatuses = ConTainerStatusRunning

				}
				if s.State.Waiting != nil {
					glog.Infof("method=%s,Message=The builder container is still waiting [%s]\n", method, s.State.Waiting)
					//statusMess.ContainerStatuses = ConTainerStatusWaiting

				}
				if s.State.Terminated != nil {
					//statusMess.ContainerStatuses = ConTainerStatusTerminated
					if s.State.Terminated.ExitCode != 0 {
						//statusMess.ContainerStatuses = ConTainerStatusError
					}
					glog.Infof("method=%s,Message=The builder container is exit abnormally [%s]\n", method, s.State.Terminated)

				}

			}
		}
	}
	return
}

const (
	ConditionTrue    string = "True"
	ConditionFalse   string = "False"
	ConditionUnknown string = "Unknown"
)
const (
	// JobComplete means the job has completed its execution.
	JobComplete string = "Complete"
	// JobFailed means the job has failed its execution.
	JobFailed string = "Failed"
)

func Int64Toint64Point(input int64) *int64 {
	tmp := new(int64)
	*tmp = int64(input)
	return tmp

}

//ESgetLogFromK8S 从Elaticsearch 获取日志失败就从kubernetes 获取日志
func (builder *ImageBuilder) ESgetLogFromK8S(namespace, podName, containerName string, ctx *context.Context) error {
	method := "ESgetLogFromK8S"
	follow := true
	previous := false

	opt := &apiv1.PodLogOptions{
		Container:  containerName,
		TailLines:  Int64Toint64Point(200),
		Previous:   previous,
		Follow:     follow,
		Timestamps: true,
	}

	readCloser, err := builder.Client.Pods(namespace).GetLogs(podName, opt).Stream()
	if err != nil {
		glog.Errorf("%s socket get pods log readCloser faile from kubernetes:==>%v\n", method, err)
		if containerName == BUILDER_CONTAINER_NAME {
			//ctx.ResponseWriter.Write([]byte(fmt.Sprintf("%s", `<font color="#ffc20e">[Enn Flow API] 日志服务暂时不能提供日志查询，请稍后再试</font><br/>`)))
		}
		return err
	}

	data := make([]byte, 1024*1024, 1024*1024)
	for {
		n, err := readCloser.Read(data)
		if nil != err {
			if err == io.EOF {
				glog.Infof("%s [Enn Flow API ] finish get log of %s.%s!\n", method, podName, containerName)
				glog.Infof("Get log successfully from kubernetes\n")
				if containerName == BUILDER_CONTAINER_NAME {
					ctx.ResponseWriter.Write([]byte(fmt.Sprintf("%s", `<font color="#ffc20e">[Enn Flow API] 日志读取结束</font><br/>`)))

				}
				return nil
			}

			glog.Errorf("get log from kubernetes failed: err:%v,", err)
			return nil
		}

		if strings.Contains(string(data[:n]), "rpc error:") {

			log := fmt.Sprintf(`<font color="#ffc20e">[%s]</font> %s <br/>`, time.Now().Format("2006/01/02 15:04:05"), string(data[:n]))
			ctx.ResponseWriter.Write([]byte(log))

		} else {

			logInfo := strings.SplitN(template.HTMLEscapeString(string(data[:n])), "\n", -1)
			for _, logline := range logInfo {
				loglineArr := strings.SplitN(logline, " ", 2)
				if len(loglineArr) == 2 {
					logTime, _ := time.Parse(time.RFC3339, loglineArr[0])
					log := fmt.Sprintf(`<font color="#ffc20e">[%s]</font> %s <br/>`, logTime.Add(8 * time.Hour).Format("2006/01/02 15:04:05"), loglineArr[1])
					ctx.ResponseWriter.Write([]byte(log))
				}

			}

			//logInfo := strings.SplitN(template.HTMLEscapeString(string(data[:n])), " ", 2)
			//logTime, _ := time.Parse(time.RFC3339, logInfo[0])
			//log := fmt.Sprintf(`<font color="#ffc20e">[%s]</font> %s <br/>`, logTime.Add(8 * time.Hour).Format("2006/01/02 15:04:05"), logInfo[1])
			//ctx.ResponseWriter.Write([]byte(log))
		}

	}

	return nil

}

func (builder *ImageBuilder) GetJobEvents(namespace, jobName, podName string) (*apiv1.EventList, error) {
	method := "GetJobEvents"
	var eventList *apiv1.EventList
	fieldSelector, err := fields.ParseSelector(fmt.Sprintf("involvedObject.kind=Job,involvedObject.name=%s", jobName))
	if nil != err {
		glog.Errorf("%s: Failed to parse field selector: %v\n", method, err)
		return eventList, err
	}
	options := metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	}
	return builder.Client.Events(namespace).List(options)

}

func (builder *ImageBuilder) GetPodEvents(namespace, podName, typeSelector string) (*apiv1.EventList, error) {
	method := "GetPodEvents"
	var eventList *apiv1.EventList
	selector := fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", podName)
	if typeSelector != "" {
		selector = fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s,%s", podName, typeSelector)
	}
	fieldSelector, err := fields.ParseSelector(selector)
	if nil != err {
		glog.Errorf("%s: Failed to parse field selector: %v\n", method, err)
		return eventList, err
	}

	options := metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	}
	return builder.Client.Events(namespace).List(options)

}

func (builder *ImageBuilder) GetJob(namespace, jobName string) (*v1beta1.Job, error) {

	return builder.Client.BatchV1Client.Jobs(namespace).Get(jobName, v1.GetOptions{})

}

func (builder *ImageBuilder) StopJob(namespace, jobName string, forced bool, succeeded int32) (*v1beta1.Job, error) {

	job, err := builder.GetJob(namespace, jobName)
	if err != nil {
		return job, err
	}
	glog.Infof("Will stop the job %s\n", jobName)
	job.Spec.Parallelism = Int32Toint32Point(0)
	if forced {
		//parallelism设为0，pod会被自动删除，但job会保留 *****
		//用来判断是否手动停止
		job.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] = "true"
	} else {
		job.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] = "Timeout-OrRunFailed"
	}

	//job watcher用来获取运行结果 失败的时候 会加个label 标识失败 1表示手动停止 0 表示由于某种原因自动执行失败
	job.ObjectMeta.Labels["enncloud-builder-succeed"] = fmt.Sprintf("%d", succeeded)

	return builder.Client.BatchV1Client.Jobs(namespace).Update(job)

}

func Int32Toint32Point(input int32) *int32 {
	tmp := new(int32)
	*tmp = int32(input)
	return tmp

}

func SplitRepo(repoUrl string) string {
	gitRepoInfo := strings.Split(repoUrl, "/")

	gitRepoInfoLen := len(gitRepoInfo)

	projectName := strings.Split(gitRepoInfo[gitRepoInfoLen-1], ".")[0]

	return projectName
}
