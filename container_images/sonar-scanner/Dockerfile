FROM alpine:3.6
RUN echo "https://mirror.tuna.tsinghua.edu.cn/alpine/v3.6/main/" > /etc/apk/repositories && \
    apk update && apk add bash && \
    apk add supervisor openssh  curl  && \
    apk add su-exec  && \
    rm -rf /var/cache/apk/*
COPY jdk1.8.0_144 /opt/java/jdk1.8.0_144
COPY run.sh /
COPY sonar-scanner-2.8 /opt/sonar/sonar-scanner-2.8
ENV JAVA_HOME /opt/java/jdk1.8.0_144
ENV CLASSPATH .:${JAVA_HOME}/jre/lib/rt.jar:${JAVA_HOME}/lib/dt.jar:${JAVA_HOME}/lib/tools.jar
ENV SONAR_SCANNER_HOME /opt/sonar/sonar-scanner-2.8
ENV PATH $PATH:${JAVA_HOME}/bin:${SONAR_SCANNER_HOME}/bin:${PATH}
WORKDIR /app/
CMD ["/run.sh"]