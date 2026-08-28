pipeline {
    agent any

    options {
        disableConcurrentBuilds()
        timestamps()

        buildDiscarder(
            logRotator(numToKeepStr: '20')
        )

        timeout(
            time: 20,
            unit: 'MINUTES'
        )
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Test') {
            steps {
                sh '''
                    set -eu

                    go test ./... -count=1
                '''
            }
        }

        stage('Deploy') {
            steps {
                sshagent(credentials: ['termchat-prod-ssh']) {
                    sh '''
                        set -eu

                        echo "Deploying commit: ${GIT_COMMIT}"

                        ssh \
                            -o BatchMode=yes \
                            deploy@192.168.1.250 \
                            "/srv/termchat/scripts/deploy.sh '${GIT_COMMIT}'"
                    '''
                }
            }
        }
    }

    post {
        success {
            echo 'TermChat deployment successful.'
        }

        failure {
            echo 'TermChat deployment FAILED.'
        }
    }
}
