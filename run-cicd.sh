#/bin/bash 
tce_url=10.39.0.217
vip_url=paasdev.enncloud.cn
regip=harbor.enncloud.cn   
db_url=10.39.0.251   
docker run --restart=always --name cicd \
               -v /etc/localtime:/etc/localtime  \
               -v /paas/tce/logs:/tmp \
               -p 38090:8090 \
               -e DB_HOST=$db_url \
               -e DB_PORT=3306 \
               -e DB_NAME=tenxcloud_2_0 \
               -e DB_USER=tcepaas \
               -e DB_PASSWORD=xboU58vQbAbN \
               -e BIND_BUILD_NODE=true \
               -e DEVOPS_EXTERNAL_PROTOCOL=https \
               -e DEVOPS_HOST=$vip_url:38090 \
               -e SONAR_SCANNER=qinzhao-harbor/sonar-scanner:v2.2
               -e DEVOPS_EXTERNAL_HOST=$vip_url:38090 \
               -e EXTERNAL_ES_URL=http://paasdev.enncloud.cn:9200 \
               -e USERPORTAL_URL=https://$vip_url \
               -e FLOW_DETAIL_URL=http://$vip_url \
               -e CICD_IMAGE_BUILDER_IMAGE=enncloud/image-builder:v2.2 \
               -e CICD_REPO_CLONE_IMAGE=paas/clone-repo:v2.2 \
            -d $regip/qinzhao-harbor/cicd:v1.4 \
