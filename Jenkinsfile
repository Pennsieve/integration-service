#!groovy

ansiColor('xterm') {
    node('executor') {
        checkout scm

        def authorName  = sh(returnStdout: true, script: 'git --no-pager show --format="%an" --no-patch')
        def isMain    = env.BRANCH_NAME == "main"
        def serviceName = env.JOB_NAME.tokenize("/")[1]
        def isRealService = serviceName != "template-serverless-service"
        def runMigrations = true

        def commitHash  = sh(returnStdout: true, script: 'git rev-parse HEAD | cut -c-7').trim()
        def imageTag    = "${env.BUILD_NUMBER}-${commitHash}"

        try {
            // On non-main branches the test Postgres seed image referenced by
            // docker-compose.test.yml may not exist yet (it is built from this
            // branch's migrations). Build and push it BEFORE running tests so the
            // test stage can pull it. On main the seed image was already pushed by
            // the originating branch's build, so we just run the tests against it.
            if (!isMain) {
                stage('Build Postgres Image') {
                      sh "IMAGE_TAG=${imageTag} ENVIRONMENT=jenkins make build-postgres"
                }
            }

            stage("Run Tests") {
                sh "IMAGE_TAG=${imageTag} make test-ci"
            }

            if (!isMain) {
                stage('Build') {
                    sh "IMAGE_TAG=${imageTag} make package"
                }
            }

            if (isMain) {
                stage ('Build and Push') {
                    sh "IMAGE_TAG=${imageTag} make publish"
                }

                stage('Run Migrations') {
                    if (runMigrations) {
                        build job: "Migrations/dev-migrations/dev-${serviceName}-postgres-migrations",
                            parameters: [
                                string(name: 'IMAGE_TAG', value: imageTag)
                            ]
                    }
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
                sh "IMAGE_TAG=${imageTag} make clean-ci"
            }
        }

        slackSend(color: '#006600', message: "SUCCESSFUL: Job '${env.JOB_NAME} [${env.BUILD_NUMBER}]' (${env.BUILD_URL}) by ${authorName}")
    }
}