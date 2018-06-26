#!/usr/bin/env bash

if [ ! -z "$REPO_TYPE" ] && [ "$REPO_TYPE" == '7' ]; then

sonar-scanner -Dsonar.projectKey=$SVNPROJECT -Dsonar.projectName=$SVNPROJECT -Dsonar.projectVersion=$SVNPROJECT -Dsonar.sources=.

else

sonar-scanner -Dsonar.projectKey=$GIT_REPO -Dsonar.projectName=$GIT_REPO -Dsonar.projectVersion=$GIT_TAG -Dsonar.sources=.

fi