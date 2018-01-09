package models

import (
	"os"
	"dev-flows-api-golang/modules/client"
	"github.com/astaxie/beego/context"
	"github.com/golang/glog"
	k8sWatch "k8s.io/client-go/1.4/pkg/watch"
	"io"
	//"text/template"
	"fmt"
	"time"
	//"encoding/json"
	v1beta1 "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"github.com/googollee/go-socket.io"
	"html/template"
	//"dev-flows-api-golang/util/rand"

	//v1 "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	apiv1 "k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/labels"
	"k8s.io/client-go/1.4/pkg/fields"
	"strings"
	"dev-flows-api-golang/models/common"
	"encoding/json"
)

const (
	SCM_CONTAINER_NAME         = "enn-scm"
	BUILDER_CONTAINER_NAME     = "enn-builder"
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

	if len(buildInfo.Command) != 0 && buildInfo.Type != 3 {
		jobContainer.Command = buildInfo.Command
	} else if len(buildInfo.Command) == 0 && buildInfo.Type != 3 {
		jobContainer.Command = buildInfo.Build_command
	}

	if buildInfo.Type == 3 {
		jobContainer.WorkingDir = "/"
	} else {
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

	//构造init container
	jobTemplate.Spec.Template.Spec.InitContainers = make([]apiv1.Container, 0)
	initContainer := apiv1.Container{
		Name:  SCM_CONTAINER_NAME,
		Image: buildInfo.ScmImage,
		//Image:           "harbor.enncloud.cn/qinzhao-harbor/clone-repo:v2.2",
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

	//==================用来标识是否是构建镜像,用来清除构建缓存
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

	glog.Infof("=========jobContainer.Env:%v\n", jobContainer.Env)
	jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, jobContainer)
	//dataJob, _ := json.Marshal(jobTemplate)
	//glog.V(1).Infof("%s ============>>jobTemplate=[%v]\n", method, string(dataJob))

	return builder.Client.BatchClient.Jobs(buildInfo.Namespace).Create(jobTemplate)

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
	pod, err := builder.GetPod(namespace, jobName, buildId[0])
	if err != nil {
		glog.Errorf("%s get pod name failed:====> %v\n", method, err)
		return "", err
	}

	return pod.ObjectMeta.Name, nil

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
	time.Sleep(5 * time.Second)
	listOptions := api.ListOptions{
		LabelSelector: labelsSel,
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

func (builder *ImageBuilder) WaitForJob(namespace, jobName string, buildWithDependency bool) StatusMessage {
	var statusMessage StatusMessage
	method := "models/ImageBuilder/WaitForJob"
	job, err := builder.Client.BatchClient.Jobs(namespace).Get(jobName)
	if err != nil {
		glog.Errorf("%s get job %s from kubernetes failed:%v\n", method, jobName, err)
		statusMessage.Phase = PodUnknown
		statusMessage.JobStatus.Succeeded = 0
		statusMessage.JobStatus.Failed = 0
		statusMessage.JobStatus.Active = 0
		statusMessage.JobStatus.Status = ConditionUnknown
		statusMessage.JobStatus.JobConditionType = JobFailed
		statusMessage.JobStatus.Message = "get job failed"
		statusMessage.JobStatus.Reason = "get job failed"
		return statusMessage
	}

	if job == nil {
		glog.Errorf("%s %s \n", method, "cannot found "+jobName+"job of the namespace="+namespace)
		statusMessage.Phase = PodFailed
		statusMessage.JobStatus.Succeeded = 0
		statusMessage.JobStatus.Failed = 0
		statusMessage.JobStatus.Active = 0
		statusMessage.JobStatus.Status = ConditionUnknown
		statusMessage.JobStatus.JobConditionType = JobFailed
		statusMessage.JobStatus.Message = "get job failed"
		statusMessage.JobStatus.Reason = "get job failed"
		return statusMessage
	}

	//if buildWithDependency {
	//	status, err := builder.WatchPod(namespace, jobName)
	//	if err != nil {
	//		glog.Errorf("%s %s \n", method, " WatchPod of the "+jobName+" job's namespace="+namespace)
	//		statusMessage.Unknown = 1
	//		return statusMessage
	//	}
	//
	//	jobInfo, err := builder.GetJob(namespace, jobName)
	//	if err != nil {
	//		glog.Errorf("%s get job %s failed:%v\n", method, jobName, err)
	//	}
	//
	//	if jobInfo.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] != "true" {
	//		//如果为手动停止，在结果中添加标记
	//		status.ForcedStop = true
	//	} else if status.Succeeded > 0 {
	//		//如果未停止且成功，则自动停止job
	//		//执行失败时，外层调用会负责停止job
	//		_, err := builder.StopJob(namespace, jobName, true, status.Succeeded)
	//		if err != nil {
	//			glog.Errorf("%s stop job failed:%v\n", method, err)
	//		}
	//	}
	//
	//	return status
	//}

	WatchRespData, err := builder.WatchJob(namespace, jobName)
	if err != nil {
		glog.Errorf("%s WatchJob failed:%v\n", method, err)
	}

	glog.Infof("WatchJob result:%#v\n", WatchRespData)

	statusMessage.JobStatus = WatchRespData
	return statusMessage
}

// These are the valid statuses of pods.
const (
	// PodPending means the pod has been accepted by the system, but one or more of the containers
	// has not been started. This includes time before being bound to a node, as well as time spent
	// pulling images onto the host.
	PodPending string = "Pending"
	// PodRunning means the pod has been bound to a node and all of the containers have been started.
	// At least one container is still running or is in the process of being restarted.
	PodRunning string = "Running"
	// PodSucceeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0, and the system is not going to restart any of these containers.
	PodSucceeded string = "Succeeded"
	// PodFailed means that all containers in the pod have terminated, and at least one container has
	// terminated in a failure (exited with a non-zero exit code or was stopped by the system).
	PodFailed string = "Failed"
	// PodUnknown means that for some reason the state of the pod could not be obtained, typically due
	// to an error in communicating with the host of the pod.
	PodUnknown string = "Unknown"
)

// These are valid conditions of pod.
const (
	// PodScheduled represents status of the scheduling process for this pod.
	PodScheduled string = "PodScheduled"
	// PodReady means the pod is able to service requests and should be added to the
	// load balancing pools of all matching services.
	PodReady string = "Ready"
	// PodInitialized means that all init containers in the pod have started successfully.
	PodInitialized string = "Initialized"
	// PodReasonUnschedulable reason in PodScheduled PodCondition means that the scheduler
	// can't schedule the pod right now, for example due to insufficient resources in the cluster.
	PodReasonUnschedulable string = "Unschedulable"

	ConTainerStatusRunning    string = "Running"
	ConTainerStatusWaiting    string = "Waiting"
	ConTainerStatusTerminated string = "Terminated"
	ConTainerStatusError      string = "Error"
)

type StatusMessage struct {
	Type string `json:"type" protobuf:"bytes,1,opt,name=type,casttype=PodConditionType"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status string `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	// Last time we probed the condition. Pending
	Phase   string
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'OutOfDisk'
	// +optional
	Reason    string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	JobStatus WatchJobRespData //job的状态
}

type PodStatus struct {
	//PodScheduled Ready  Initialized Unschedulable
	Type string `json:"type" protobuf:"bytes,1,opt,name=type,casttype=PodConditionType"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status string `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	//  Pending Running  Succeeded Failed Unknown
	Phase   string
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'OutOfDisk'
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	//容器状态
	ContainerStatuses     string
	InitContainerStatuses string
}

func (builder *ImageBuilder) WatchPod(namespace, jobName string) (PodStatus, error) {
	method := "WatchPod"
	var status PodStatus
	labelsStr := fmt.Sprintf("job-name=%s", jobName)
	labelSelector, err := labels.Parse(labelsStr)
	if err != nil {
		glog.Errorf("%s label parse failed: %v\n", method, err)
		status.Phase = PodUnknown
		status.Message = "label Parse failed"
		status.Reason = "label Parse failed"
		status.Type = PodReasonUnschedulable
		status.Status = ConditionUnknown
		return status, err
	}

	podName := ""
	listOptions := api.ListOptions{
		LabelSelector: labelSelector,
		Watch:         true,
	}
	// 请求watch api监听pod发生的事件
	watchInterface, err := builder.Client.Pods(namespace).Watch(listOptions)
	if err != nil {
		glog.Errorf("%s get pod watchInterface failed: %v\n", method, err)
		status.Phase = PodUnknown
		status.Message = "get pod watchInterface failed"
		status.Reason = "get pod watchInterface failed"
		status.Type = PodReasonUnschedulable
		status.Status = ConditionUnknown
		return status, err
	}

	for {
		select {
		case event, isOpen := <-watchInterface.ResultChan():
			if !isOpen {
				glog.Warningf("the pod watch the chan is closed\n")
				status.Phase = PodUnknown
				status.Message = "the pod watch the chan is closed"
				status.Reason = "the pod watch the chan is closed"
				status.Type = PodReasonUnschedulable
				status.Status = ConditionUnknown
				break
			}
			glog.Infof("The pod event type=%s\n", event.Type)
			pod, parseIsOk := event.Object.(*apiv1.Pod)
			if !parseIsOk {
				glog.Errorf("get pod event failed:断言失败\n")
				status.Phase = PodUnknown
				status.Message = "get pod event failed 断言失败"
				status.Reason = "get pod event failed 断言失败"
				status.Type = PodReasonUnschedulable
				status.Status = ConditionUnknown
				continue
			}
			//保存首次收到的事件所属的pod名称
			if podName == "" {
				podName = pod.ObjectMeta.Name
			} else if pod.ObjectMeta.Name != podName {
				//收到其他pod事件时不处理
				continue
			}

			if event.Type == k8sWatch.Deleted {
				glog.Errorf("%s pod of job %s is deleted with final status: %#v\n", method, jobName, pod.Status)
				//收到deleted事件，pod可能被删除
				ContainerStatus := TranslatePodStatus(pod.Status)
				status.ContainerStatuses = ContainerStatus.ContainerStatuses
				status.InitContainerStatuses = ContainerStatus.InitContainerStatuses
				status.Phase = PodFailed
				break
			} else if len(pod.Status.ContainerStatuses) > 0 {
				glog.Infof("%s get pod failed %v\n", method, pod.Status)
				//存在containerStatuses时
				ContainerStatus := TranslatePodStatus(pod.Status)
				status.ContainerStatuses = ContainerStatus.ContainerStatuses
				status.InitContainerStatuses = ContainerStatus.InitContainerStatuses
				status.Phase = string(pod.Status.Phase)

			}
			if event.Type == k8sWatch.Error {
				glog.Errorf("%s %s %v\n", method, "call watch api of pod of "+jobName+" error:", event.Object)
				continue
			}
			//成功
			if pod.Status.Phase == apiv1.PodSucceeded {
				status.Phase = PodSucceeded
				status.Message = fmt.Sprintf("pod执行成功:%s", pod.Status.Message)
				status.Reason = fmt.Sprintf("pod执行成功:%s", pod.Status.Reason)
				status.Type = fmt.Sprintf("%s", pod.Status.Conditions)
				status.Status = fmt.Sprintf("%s", pod.Status.Conditions)
				break
			} else if pod.Status.Phase == apiv1.PodFailed {
				//创建失败
				status.Phase = PodFailed
				status.Message = fmt.Sprintf("pod执行失败:%s", pod.Status.Message)
				status.Reason = fmt.Sprintf("pod执行失败:%s", pod.Status.Reason)
				status.Type = fmt.Sprintf("%s", pod.Status.Conditions)
				status.Status = fmt.Sprintf("%s", pod.Status.Conditions)
				break
			}

		}

	}

	return status, nil

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
	options := api.ListOptions{
		FieldSelector: fieldSelector,
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
func TranslatePodStatus(status apiv1.PodStatus) (statusMess PodStatus) {
	method := "TranslatePodStatus"
	//获取SCM 容器的状态
	if len(status.InitContainerStatuses) != 0 {
		for _, s := range status.InitContainerStatuses {
			if SCM_CONTAINER_NAME == s.Name {
				if s.State.Running != nil {
					glog.Infof("method=%s,Message=The scm container is still running [%s]\n", method, s.State.Running)
					statusMess.InitContainerStatuses = ConTainerStatusRunning
				}
				if s.State.Waiting != nil {
					glog.Infof("method=%s,Message=The scm container is still waiting [%s]\n", method, s.State.Waiting)
					statusMess.InitContainerStatuses = ConTainerStatusWaiting
				}
				if s.State.Terminated != nil {
					statusMess.InitContainerStatuses = ConTainerStatusTerminated
					if s.State.Terminated.ExitCode != 0 {
						statusMess.ContainerStatuses = ConTainerStatusError
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
					statusMess.ContainerStatuses = ConTainerStatusRunning

				}
				if s.State.Waiting != nil {
					glog.Infof("method=%s,Message=The builder container is still waiting [%s]\n", method, s.State.Waiting)
					statusMess.ContainerStatuses = ConTainerStatusWaiting

				}
				if s.State.Terminated != nil {
					statusMess.ContainerStatuses = ConTainerStatusTerminated
					if s.State.Terminated.ExitCode != 0 {
						statusMess.ContainerStatuses = ConTainerStatusError
					}
					glog.Infof("method=%s,Message=The builder container is exit abnormally [%s]\n", method, s.State.Terminated)

				}

			}
		}
	}
	return
}

type WatchJobRespData struct {
	// JobComplete means the job has = "Complete"  has = "Failed" completed its execution.
	JobConditionType string
	// Status of the condition, one of True, False, Unknown.
	Status string
	// The number of actively running pods.
	// +optional
	Active int32 `json:"active,omitempty" protobuf:"varint,4,opt,name=active"`
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
	// The number of pods which reached phase Succeeded.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty" protobuf:"varint,5,opt,name=succeeded"`

	// The number of pods which reached phase Failed.
	// +optional
	Failed     int32 `json:"failed,omitempty" protobuf:"varint,6,opt,name=failed"`
	ForcedStop bool `json:"forcedStop"` //用来标识是否被强制停止
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

// WatchJob  watch  the job event
func (builder *ImageBuilder) WatchJob(namespace, jobName string) (WatchJobRespData, error) {
	method := "WatchJob"
	var watchRespData WatchJobRespData

	glog.Infof("%s begin watch job jobName=[%s]  namespace=[%s]\n", method, jobName, namespace)
	opts := api.ListOptions{
		Watch: true,
	}

	watchInterface, err := builder.Client.BatchClient.Jobs(namespace).Watch(opts)
	if err != nil {
		glog.Errorf("%s  %s\n", method, ">>>>>>断言失败<<<<<<")
		watchRespData.Succeeded = 0
		watchRespData.Failed = 1
		watchRespData.Active = 0
		watchRespData.Status = ConditionUnknown
		watchRespData.JobConditionType = JobFailed
		watchRespData.Message = "get job watchInterface failed"
		watchRespData.Reason = "get job watchInterface failed"
		return watchRespData, err
	}

	for {
		select {
		case event, isOpen := <-watchInterface.ResultChan():
			if isOpen == false {
				glog.Errorf("%s the watch job chain is close\n", method)
				watchRespData.Succeeded = 0
				watchRespData.Failed = 1
				watchRespData.Active = 0
				watchRespData.Status = ConditionUnknown
				watchRespData.JobConditionType = JobFailed
				watchRespData.Message = " the watch job chain is close"
				watchRespData.Reason = " the watch job chain is close"
				return watchRespData, fmt.Errorf("%s the job watch chain is close", method)
			}
			dm, parseIsOk := event.Object.(*v1beta1.Job)
			if false == parseIsOk {
				glog.Errorf("%s job %s\n", method, ">>>>>>断言失败<<<<<<")
				watchRespData.Succeeded = 0
				watchRespData.Failed = 1
				watchRespData.Active = 0
				watchRespData.Status = ConditionUnknown
				watchRespData.JobConditionType = JobFailed
				watchRespData.Message = "event job 断言 failed"
				watchRespData.Reason = "event job 断言 failed"
				return watchRespData, fmt.Errorf(">>>>>>断言失败<<<<<<")
			}

			glog.Infof("%s job event.Type=%s\n", method, event.Type)
			glog.Infof("%s job event.Status=%#v\n", method, dm.Status)

			if dm.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] == "true" {
				watchRespData.ForcedStop = true

			}
			if event.Type == k8sWatch.Added {
				//收到deleted事件，job可能被第三方删除
				glog.Infof("%s  %s\n", method, " 收到Add事件")
				watchRespData.Succeeded = dm.Status.Succeeded
				watchRespData.Failed = dm.Status.Failed
				watchRespData.Active = dm.Status.Active
				if len(dm.Status.Conditions) != 0 {
					watchRespData.Status = string(dm.Status.Conditions[0].Status)
					watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
					watchRespData.Message = dm.Status.Conditions[0].Message
					watchRespData.Reason = dm.Status.Conditions[0].Reason
				} else {
					watchRespData.Status = ConditionUnknown
					watchRespData.JobConditionType = JobFailed
					watchRespData.Message = "收到add事件"
					watchRespData.Reason = "收到add事件"
				}
				//成功时并且已经完成时
			} else if event.Type == k8sWatch.Deleted {
				//收到deleted事件，job可能被第三方删除
				glog.Errorf("%s  %s\n", method, " 收到deleted事件，job可能被第三方删除")
				watchRespData.Succeeded = dm.Status.Succeeded
				watchRespData.Failed = dm.Status.Failed
				watchRespData.Active = dm.Status.Active
				if len(dm.Status.Conditions) != 0 {
					watchRespData.Status = string(dm.Status.Conditions[0].Status)
					watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
					watchRespData.Message = dm.Status.Conditions[0].Message
					watchRespData.Reason = dm.Status.Conditions[0].Reason
				} else {
					watchRespData.Status = ConditionUnknown
					watchRespData.JobConditionType = JobFailed
					watchRespData.Message = "收到deleted事件，job可能被第三方删除"
					watchRespData.Reason = "收到deleted事件，job可能被第三方删除"
				}
				return watchRespData, fmt.Errorf("收到deleted事件，job可能被第三方删除")
				//成功时并且已经完成时
			} else if dm.Status.Succeeded >= 1 &&
				dm.Status.CompletionTime != nil && len(dm.Status.Conditions) != 0 {

				//job执行成功
				glog.Infof("%s the job [%s] run success\n", method, jobName)
				watchRespData.Succeeded = dm.Status.Succeeded
				watchRespData.Failed = dm.Status.Failed
				watchRespData.Active = dm.Status.Active
				if len(dm.Status.Conditions) != 0 {
					watchRespData.Status = string(dm.Status.Conditions[0].Status)
					watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
					watchRespData.Message = dm.Status.Conditions[0].Message
					watchRespData.Reason = dm.Status.Conditions[0].Reason
				}

				return watchRespData, nil
				//} else if dm.Status.Failed >=1 && dm.Spec.Completions == Int32Toint32Point(1) &&
				//	dm.Status.CompletionTime == nil && dm.Status.Succeeded==0{
			} else if dm.Status.Failed >= 1 {
				watchRespData.Succeeded = dm.Status.Succeeded
				watchRespData.Failed = dm.Status.Failed
				watchRespData.Active = dm.Status.Active
				//job执行失败
				glog.Warningf("%s the job [%s] run failed\n", method, jobName)
				if len(dm.Status.Conditions) != 0 {
					watchRespData.Status = string(dm.Status.Conditions[0].Status)
					watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
					watchRespData.Message = dm.Status.Conditions[0].Message
					watchRespData.Reason = dm.Status.Conditions[0].Reason
				}
				return watchRespData, fmt.Errorf("job run failed")
				//手动停止job
			} else if dm.Spec.Parallelism == Int32Toint32Point(0) {
				watchRespData.ForcedStop = true
				//有依赖服务，停止job时 不是手动停止 1 表示手动停止
				if dm.ObjectMeta.Labels["enncloud-builder-succeed"] != "1" {
					watchRespData.Succeeded = dm.Status.Succeeded
					watchRespData.Failed = dm.Status.Failed
					watchRespData.Active = dm.Status.Active
					//job停止成功
					if len(dm.Status.Conditions) != 0 {
						watchRespData.Status = string(dm.Status.Conditions[0].Status)
						watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
						watchRespData.Message = dm.Status.Conditions[0].Message
						watchRespData.Reason = dm.Status.Conditions[0].Reason
					} else {
						watchRespData.Status = ConditionUnknown
						watchRespData.JobConditionType = JobFailed
						watchRespData.Message = "停止job成功"
						watchRespData.Reason = "停止job成功"
					}
					return watchRespData, fmt.Errorf("用户停止了构建任务")
					//没有依赖服务时
				} else {
					if len(dm.Status.Conditions) != 0 {
						watchRespData.Status = string(dm.Status.Conditions[0].Status)
						watchRespData.JobConditionType = string(dm.Status.Conditions[0].Type)
						watchRespData.Message = dm.Status.Conditions[0].Message
						watchRespData.Reason = dm.Status.Conditions[0].Reason
					} else {
						watchRespData.Status = ConditionUnknown
						watchRespData.JobConditionType = JobFailed
						watchRespData.Message = "停止job成功"
						watchRespData.Reason = "停止job成功"
					}
					return watchRespData, fmt.Errorf("job执行失败程序发送了停止构建任务的命令")
				}
			}

		}
	}
	return watchRespData, nil

}

func Int64Toint64Point(input int64) *int64 {
	tmp := new(int64)
	*tmp = int64(input)
	return tmp

}

//ESgetLogFromK8S 从Elaticsearch 获取日志失败就从kubernetes 获取日志
func (builder *ImageBuilder) ESgetLogFromK8S(namespace, podName, containerName string, ctx *context.Context) {
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
			ctx.ResponseWriter.Write([]byte(fmt.Sprintf("%s", `<font color="#ffc20e">[Enn Flow API] 日志服务暂时不能提供日志查询，请稍后再试</font><br/>`)))
		}
		return
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
				return
			}
			if containerName == BUILDER_CONTAINER_NAME {
				ctx.ResponseWriter.Write([]byte(fmt.Sprintf("%s", `<font color="#ffc20e">[Enn Flow API] 日志服务暂时不能提供日志查询，请稍后再试</font><br/>`)))

			}
			glog.Errorf("get log from kubernetes failed: err:%v,", err)
			return
		}
		glog.Infof("==========>>%s\n", template.HTMLEscapeString(string(data[:n])))
		logInfo := strings.SplitN(template.HTMLEscapeString(string(data[:n])), " ", 2)
		logTime, _ := time.Parse(time.RFC3339, logInfo[0])
		log := fmt.Sprintf(`<font color="#ffc20e">[%s]</font> %s <br/>`, logTime.Add(8 * time.Hour).Format("2006/01/02 15:04:05"), logInfo[1])
		ctx.ResponseWriter.Write([]byte(log))

	}

	return

}

func FormatLog(data string) (buildLogs string) {
	glog.Infof("log data :=======>%s\n", data)
	var logdataHtml []string
	if data != "" {
		dataLine := strings.Split(data, "\n")
		dataLineLne := len(dataLine)
		for index, d := range dataLine {
			if index == dataLineLne-1 {
				break
			}
			logdataHtml = strings.Split(d, " ")
			buildLogs += `<font color="#ffc20e">['` + logdataHtml[0] + `']</font> ` + logdataHtml[1] + `<br/>`
		}

	}

	return
}
func (builder *ImageBuilder) GetJobEvents(namespace, jobName, podName string) (*apiv1.EventList, error) {
	method := "GetJobEvents"
	var eventList *apiv1.EventList
	fieldSelector, err := fields.ParseSelector(fmt.Sprintf("involvedObject.kind=Job,involvedObject.name=%s", jobName))
	if nil != err {
		glog.Errorf("%s: Failed to parse field selector: %v\n", method, err)
		return eventList, err
	}
	options := api.ListOptions{
		FieldSelector: fieldSelector,
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

	options := api.ListOptions{
		FieldSelector: fieldSelector,
	}
	return builder.Client.Events(namespace).List(options)

}

func (builder *ImageBuilder) GetJob(namespace, jobName string) (*v1beta1.Job, error) {

	return builder.Client.BatchClient.Jobs(namespace).Get(jobName)

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

	return builder.Client.BatchClient.Jobs(namespace).Update(job)

}

func Int32Toint32Point(input int32) *int32 {
	tmp := new(int32)
	*tmp = int32(input)
	return tmp

}
