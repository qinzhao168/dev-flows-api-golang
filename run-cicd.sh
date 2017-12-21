#/bin/bash 
tce_url=10.39.0.102
vip_url=10.39.0.102
regip=harbor.enncloud.cn   
db_url=10.39.0.102   
docker run --restart=always --name cicd \
               -v /etc/localtime:/etc/localtime  \
               -v /paas/tce/logs:/tmp \
               -p 48090:8090 \
               -e DB_HOST=$db_url \
               -e DB_PORT=3306 \
               -e DB_NAME=tenxcloud_2_0 \
               -e DB_USER=root \
               -e DB_PASSWORD=enN12345 \
               -e BIND_BUILD_NODE=true \
               -e DEVOPS_EXTERNAL_PROTOCOL=http \
               -e DEVOPS_HOST=$vip_url:48090 \
               -e DEVOPS_EXTERNAL_HOST=$vip_url:48090 \
               -e EXTERNAL_ES_URL=http://paasdev.enncloud.cn:9200 \
               -e USERPORTAL_URL=http://$vip_url \
               -e FLOW_DETAIL_URL=http://$vip_url \
               -e CICD_IMAGE_BUILDER_IMAGE=enncloud/image-builder:v2.2 \
               -e CICD_REPO_CLONE_IMAGE=qinzhao-harbor/clone-repo:v2.2 \
            -d $regip/qinzhao-harbor/cicd:v1.4 \
          /run.sh -u root -p enN12345 -H $db_url -P 3306