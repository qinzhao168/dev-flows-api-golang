package controllers

import (
	//"k8s.io/client-go/pkg/api"
	v1batch "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"dev-flows-api-golang/modules/client"
	"github.com/golang/glog"
	"fmt"
	"time"
	"dev-flows-api-golang/models/common"
	"os"
	"strconv"
	"dev-flows-api-golang/models"
	"strings"
)

type JobStatusCount struct {
	StopJobSount   int
	SucceededCount int
	FailedCount    int
	Running        int
	ToOldJob       int
}

func init() {
	go deleteCICDJobs()
}

//默认值是10min执行一次清除job操作
func deleteCICDJobs() {
	BeginDeleteJob()
	DeleteNsfCronJob()
	var err error
	var intervalTime int
	INTERVAL_TIME := os.Getenv("INTERVAL_TIME")
	if INTERVAL_TIME == "" {
		intervalTime = 10
	} else {
		intervalTime, err = strconv.Atoi(INTERVAL_TIME)
		if err != nil {
			fmt.Printf("strconv.Atoi failed %v\n", err)
			return
		}
	}

	t := time.NewTicker(time.Duration(intervalTime) * time.Minute)
	glog.Infof("Delete job server start time = %s \n", time.Now())
	for {
		select {
		case <-t.C:
			fmt.Printf("Delete job begin time = %s\n", time.Now())
			BeginDeleteJob()
			DeleteNsfCronJob()
			glog.Infof("Delete  job end time = %s\n", time.Now())
		}

	}
}

func BeginDeleteJob() {
	var jobStatusCount JobStatusCount
	//get all job
	jobList, err := getAllJobsInAllNamespaces()
	if err != nil {
		glog.Errorf("Get jobList failed:Err:%v\n", err)
		return
	}

	if len(jobList.Items) == 0 {
		return
	}

	glog.Infof("jobList.Items.length=%d\n", len(jobList.Items))

	for _, job := range jobList.Items {

		//watchOneJob(job.ObjectMeta.Namespace, job.ObjectMeta.Name)

		if succeeded(job) {
			jobStatusCount.SucceededCount++
		}

		if failed(job) {
			jobStatusCount.FailedCount++

		}

		if running(job) {
			jobStatusCount.Running++
		}

		if CanDeleteJob(job) || deleteImmediately(job) {
			jobStatusCount.StopJobSount++
			err := deleteOneJob(job.ObjectMeta.Namespace, job.ObjectMeta.Name)
			if err != nil {
				glog.Errorf("delete job %s failed:%v", job.ObjectMeta.Name, err)
			}
		}

		if tooOld(job) {
			jobStatusCount.ToOldJob++
			jobStatusCount.StopJobSount++
			err := deleteOneJob(job.ObjectMeta.Namespace, job.ObjectMeta.Name)
			if err != nil {
				glog.Errorf("delete job %s failed:%v", job.ObjectMeta.Name, err)
			}
		}

	}

	glog.Infof("%d manual  stop or can delete jobs\n", jobStatusCount.StopJobSount)
	glog.Infof("%d running jobs\n", jobStatusCount.Running)
	glog.Infof("%d succeeded jobs\n", jobStatusCount.SucceededCount)
	glog.Infof("%d failed jobs\n", jobStatusCount.FailedCount)
	glog.Infof("%d too old jobs\n", jobStatusCount.ToOldJob)

}

func succeeded(job v1batch.Job) bool {

	return job.Status.Succeeded >= 1
}

func failed(job v1batch.Job) bool {

	return job.Status.Failed >= 1
}

func running(job v1batch.Job) bool {

	return job.Status.Active >= 1
}

func getAllJobsInAllNamespaces() (*v1batch.JobList, error) {
	method := "getAllJobsInAllNamespaces"
	var list *v1batch.JobList
	labelsStr := fmt.Sprintf("system/jobType=%s", "devflows")
	labelsSel, err := labels.Parse(labelsStr)
	if err != nil {
		glog.Errorf("%s label parse failed==>:%v\n", method, err)
		return list, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labelsSel.String(),
	}

	return client.KubernetesClientSet.BatchV1Client.Jobs("").List(listOptions)

}

func getAllJobsOfCronJobInDefaultNamespace() (*v1batch.JobList, error) {
	listOptions := metav1.ListOptions{}
	return client.KubernetesClientSet.BatchV1Client.Jobs("default").List(listOptions)

}

func DeleteNsfCronJob() {

	//get all job
	jobList, err := getAllJobsOfCronJobInDefaultNamespace()
	if err != nil {
		glog.Errorf("Get CronJob jobList failed:Err:%v\n", err)
		return
	}

	if len(jobList.Items) == 0 {
		return
	}

	glog.Infof("jobList.Items.length=%d\n", len(jobList.Items))

	for _, job := range jobList.Items {

		if CanDeleteJob(job) || tooOld(job) {

			err := deleteOneCronJobInNamespace(job)
			if err != nil {
				glog.Errorf("deleteOneCronJobInNamespace job %s failed:%v", job.ObjectMeta.Name, err)
			}
		}

	}

}

func deleteOneCronJobInNamespace(job v1batch.Job) error {

	if strings.Contains(job.Labels["job-name"], "nfs-") {
		for _, ownerReference := range job.OwnerReferences {
			if ownerReference.Kind == "CronJob" {
				return client.KubernetesClientSet.BatchV1Client.Jobs(job.Namespace).Delete(job.ObjectMeta.Name, nil)
			}

		}
	}

	return nil

}

//int(time.Now().Sub(c.Audit.StartTime) / time.Microsecond)
func CanDeleteJob(job v1batch.Job) bool {

	return time.Now().Sub(job.Status.StartTime.Time) >= 4*time.Hour

}

//can delete immediately job labelsStr := fmt.Sprintf("job-name=%s", jobName)

func deleteJobRelatedPods(namespace, label string) error {
	method := "deleteJobRelatedPods"
	labelsSel, err := labels.Parse(label)
	if err != nil {
		glog.Errorf("%s label parse failed==>:%v\n", method, err)
		return err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labelsSel.String(),
	}
	return client.KubernetesClientSet.Pods(namespace).DeleteCollection(nil, listOptions)
}

func deleteImmediately(job v1batch.Job) bool {

	return job.Spec.Parallelism == Int32Toint32Point(0) &&
		job.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] == "true"
}

//too old job
func tooOld(job v1batch.Job) bool {
	return time.Now().Sub(job.Status.StartTime.Time) > 24*7*time.Hour

}

func deleteOneJobInNamespace(namespace, jobName string) error {
	var OrphanDependents bool
	OrphanDependents = true
	options := &metav1.DeleteOptions{
		//GracePeriodSeconds: Int64Toint64Point(1),
		OrphanDependents: &OrphanDependents,
	}
	labelsStr := fmt.Sprintf("job-name=%s", jobName)

	err := deleteJobRelatedPods(namespace, labelsStr)
	if err != nil {
		glog.Errorf("deleteJobRelatedPods failed:%v\n", err)
		return err
	}
	return client.KubernetesClientSet.BatchV1Client.Jobs(namespace).Delete(jobName, options)

	//labelsStr := fmt.Sprintf("job-name=%s", jobName)
	//return client.KubernetesClientSet.BatchClient.Delete().Namespace(namespace).Resource("jobs").Name(jobName).Do().Error()
	////err := client.KubernetesClientSet.Delete().Resource("jobs").Namespace(namespace).Name(jobName).Do().Error()
	//if err != nil {
	//	glog.Infof("delete job failed:err:%v\n", err)
	//}
	//return deleteJobRelatedPods(namespace, labelsStr)

}

func deleteOneJob(namespace, jobName string) error {

	return deleteOneJobInNamespace(namespace, jobName)
}

func watchOneJob(namespace, jobName string) {
	job, status := Watch(namespace, jobName)

	if job != nil {

		syncToDB(job, status)

	}

}

var unknownStatus = -1

func Watch(namespace, jobName string) (*v1batch.Job, int) {
	method := "WatchJob"

	labelsStr := fmt.Sprintf("job-name=%s", jobName)
	labelsSel, err := labels.Parse(labelsStr)
	if err != nil {
		glog.Errorf("%s label parse failed==>:%v\n", method, err)
		return nil, unknownStatus
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelsSel.String(),
		Watch:         true,
	}

	watchInterface, err := client.KubernetesClientSet.BatchV1Client.Jobs(namespace).Watch(listOptions)
	if err != nil {
		glog.Errorf("%s,err: %v\n", method, err)
		return nil, unknownStatus
	}

	glog.Infof("%s delete begin watch job jobName=[%s]  namespace=[%s],watchInterface=%#v\n", method, jobName, namespace, watchInterface.ResultChan())

	for {
		select {
		case event, isOpen := <-watchInterface.ResultChan():
			glog.Infof("%s delete begin watch job jobName=[%s]  namespace=[%s]\n", method, jobName, namespace)
			if isOpen == false {
				glog.Errorf("%s the watch job chain is close\n", method)
				return nil, unknownStatus
			}

			dm, parseIsOk := event.Object.(*v1batch.Job)
			if false == parseIsOk {
				glog.Errorf("%s job %s\n", method, ">>>>>>断言失败<<<<<<")
				return nil, unknownStatus
			}

			glog.Infof("%s job event.Type=%s\n", method, event.Type)
			glog.Infof("%s job event.Status=%#v\n", method, dm.Status)
			if dm.Status.Active >= 1 {

				glog.Infof("%s  %s status:%v\n", method, " running ", dm.Status)
				return dm, common.STATUS_BUILDING

			} else if dm.Status.Succeeded >= 1 {

				glog.Infof("%s %s,status:%v\n", method, "Succeeded", dm.Status)

				return dm, common.STATUS_SUCCESS

			} else if dm.Status.Failed >= 1 {

				glog.Infof("%s %s,status:%v\n", method, "Failed", dm.Status)
				return dm, common.STATUS_FAILED

				//手动停止job
			} else if dm.Spec.Parallelism == Int32Toint32Point(0) {
				//有依赖服务，停止job时 不是手动停止 1 表示手动停止
				if dm.ObjectMeta.Labels["enncloud-builder-succeed"] != "1" && dm.ObjectMeta.Labels[common.MANUAL_STOP_LABEL] == "true" {
					glog.Infof("%s %s,status:%v\n", method, "用户停止了构建任务", dm)
					return dm, common.STATUS_FAILED
					//没有依赖服务时
				}

			} else {
				return nil, common.STATUS_FAILED
			}
		case <-time.After(3 * time.Second):
			return nil, common.STATUS_FAILED


		}
	}

	return nil, unknownStatus
}

func syncToDB(job *v1batch.Job, status int) {

	if status == unknownStatus {
		glog.Warningf("job in unknown status, job name: %s,namespace=%s", job.ObjectMeta.Name, job.ObjectMeta.Namespace)
		return
	}

	glog.Infof("sync job status to db, job name:%s,status:%d,namespace:%s", job.ObjectMeta.Name,
		status, job.ObjectMeta.Namespace)

	flowBuildId := job.ObjectMeta.Labels["flow-build-id"]

	models.NewCiFlowBuildLogs().UpdateById(time.Now(), status, flowBuildId)

}
