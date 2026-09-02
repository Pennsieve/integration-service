#!groovy

ansiColor('xterm') {
  node('executor') {

  checkout scm

  def authorName  = sh(returnStdout: true, script: 'git --no-pager show --format="%an" --no-patch')
  def isMain    = env.BRANCH_NAME == "main"
  def serviceName = env.JOB_NAME.tokenize("/")[1]

  def commitHash  = sh(returnStdout: true, script: 'git rev-parse HEAD | cut -c-7').trim()
  def imageTag    = "${env.BUILD_NUMBER}-${commitHash}"

  try {

    stage('Run Tests') {
      withEnv(['CI=true']) {
        sh "make test-docker"
      }
    }

    if(isMain) {
      stage ('Build and Push') {
        sh "IMAGE_TAG=${imageTag} make publish"
      }

      stage('Run Migrations') {
        // single dbmigrate job/image; the binary runs the webhooks and
        // notifications schema migrations itself, one after the other
        build job: "Migrations/dev-migrations/dev-${serviceName}-postgres-migrations",
        parameters: [
          string(name: 'IMAGE_TAG', value: imageTag)
        ]
      }

      stage("Deploy") {
        build job: "service-deploy/pennsieve-non-prod/us-east-1/dev-vpc-use1/dev/${serviceName}",
        parameters: [
          string(name: 'IMAGE_TAG', value: imageTag),
          string(name: 'TERRAFORM_ACTION', value: 'apply')
        ]
      }
    }
  } catch (e) {
    slackSend(color: '#b20000', message: "FAILED: Job '${env.JOB_NAME} [${env.BUILD_NUMBER}]' (${env.BUILD_URL}) by ${authorName}")
    throw e
  } finally {
    stage("Clean Up") {
        sh "make clean"
    }
  }

  slackSend(color: '#006600', message: "SUCCESSFUL: Job '${env.JOB_NAME} [${env.BUILD_NUMBER}]' (${env.BUILD_URL}) by ${authorName}")
  }
}