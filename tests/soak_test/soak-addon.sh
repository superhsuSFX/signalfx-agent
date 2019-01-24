
checkout(){
    # Cloning of signalfx-agent repo
    git clone https://github.com/signalfx/signalfx-agent.git
    git clone https://github.com/pyenv/pyenv.git ~/.pyenv
    echo "INFO Finished checkout of signalfx-agent and pyenv repos"
}

install_packages(){
    # Installation of git and make
    sudo apt-get -y update
    sudo apt-get install -y git
    sudo apt-get install -y make build-essential libssl-dev zlib1g-dev libbz2-dev libreadline-dev libsqlite3-dev wget curl llvm libncurses5-dev xz-utils tk-dev libxml2-dev libxmlsec1-dev libffi-dev
    echo "INFO Successfully installed apt packages"
}

install_docker(){
    # Installation of Docker
    sudo apt-get remove docker docker-engine docker.io containerd runc
    sudo apt -y update
    sudo apt-get install -y apt-transport-https ca-certificates curl software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    sudo apt-get -y update
    sudo apt-get install -y docker-ce
    sudo groupadd docker
    sudo usermod -aG docker ubuntu
    source /etc/bash.bashrc
    source ~/.bashrc
    echo "INFO Successfully installed docker"
}

install_pytest(){
    # Installation of pytest and dependencies
    export PYENV_ROOT="$HOME/.pyenv"
    export PATH="$PYENV_ROOT/bin:$PATH"
    if command -v pyenv 1>/dev/null 2>&1; then eval "$(pyenv init -)";fi
    pyenv install --skip-existing 3.6.3
    pyenv global 3.6.3
    if which pip; then
        pip install --upgrade 'pip==10.0.1'
    else
        curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py
        python get-pip.py 'pip==10.0.1'
    fi
    pip install "urllib3<1.24,>=1.21.1"
    pip install -r  $AGENT_HOME/tests/requirements.txt
    pyenv rehash
    echo "INFO Successfully installed pytest"
}

build_image(){
    # Build signalfx-agent image
    cd $AGENT_HOME
    PULL_CACHE=yes AGENT_VERSION=latest make image
}

setup_test_env(){
    # Setup test environment
    echo "INFO Setup test environment"
    SRC=${1:-$AGENT_HOME}
    export BUNDLE_DIR="$SRC/bundle"
    export AGENT_BIN="$BUNDLE_DIR/bin/signalfx-agent"
    export TEST_SERVICES_DIR="$SRC/test-services"
    mkdir -p "$BUNDLE_DIR"
    cid=$(sudo docker create quay.io/signalfx/signalfx-agent-dev:latest true)
    sudo docker export $cid | tar -C "$BUNDLE_DIR" -xf -
    sudo docker rm -fv $cid
    [ -f "$AGENT_BIN" ] || (echo "$AGENT_BIN not found!" && exit 1) 
}

run_pytest() {
    # Run pytest
    echo "INFO Running pytest"
    PYTEST_OPTIONS=${1}
    WITH_SUDO=${2:-false}
    
    PYTEST_OPTIONS="$PYTEST_OPTIONS --verbose --junitxml=~/testresults/results.xml --html=~/testresults/results.html --self-contained-html"
    TESTS=${TESTS_DIR:-/home/ubuntu/signalfx-agent/tests}
    [ -f ~/.skip ] && echo "Found ~/.skip, skipping tests!" && exit 0
    [ -d "$TESTS" ] || (echo "Directory '$TESTS' not found!" && exit 1)
    mkdir -p /tmp/scratch
    mkdir -p ~/testresults
    echo "Executing test(s) from $TESTS"
    if [ "$WITH_SUDO" = "true" ]; then
        sudo -E $PYENV_ROOT/shims/pytest $PYTEST_OPTIONS $TESTS
    else
        pytest $PYTEST_OPTIONS $TESTS
    fi
}

integration_tests(){
    setup_test_env
    install_pytest
    # Integration tests test environment
    export CIRCLE_BRANCH=master
    export GIT_DIR=$AGENT_HOME.git
    MARKERS="not packaging and not installer and not k8s"
    if ! $AGENT_HOME/scripts/changes-include-dir $(find . -iname "*devstack*" -o -iname "*openstack*" | sed 's|^\./||' | grep -v '^docs/'); then
        MARKERS="$MARKERS and not openstack"
    fi
    if ! $AGENT_HOME/scripts/changes-include-dir $(find . -iname "*conviva*" | sed 's|^\./||' | grep -v '^docs/'); then
        MARKERS="$MARKERS and not conviva"
    fi
    if ! $AGENT_HOME/scripts/changes-include-dir $(find . -iname "*jenkins*" | sed 's|^\./||' | grep -v '^docs/'); then
        MARKERS="$MARKERS and not jenkins"
    fi
    export MARKERS=$MARKERS
    PYTEST_OPTIONS="-n4 -m \"$MARKERS\""
    WITH_SUDO=true
    run_pytest $PYTEST_OPTIONS $WITH_SUDO
}

k8s_integration_tests(){
    # K8s Integration Tests
    setup_test_env
    install_pytest
    export MARKERS=k8s
    PYTEST_OPTIONS="-n4 -m \"$MARKERS\" --exitfirst --k8s-version=v1.13.0 --k8s-sfx-agent=quay.io/signalfx/signalfx-agent-dev:latest --k8s-timeout=$K8S_TIMEOUT"
    WITH_SUDO=true
    run_pytest $PYTEST_OPTIONS $WITH_SUDO
}

export AGENT_HOME=/home/ubuntu/signalfx-agent

while getopts ":j:" opt; do
    case ${opt} in
        j )
            job=$OPTARG
            if [[ $job = 'checkout' ]]; then
                checkout
            elif [[ $job = 'install' ]]; then
                install_packages
                install_docker
            elif [[ $job = 'build' ]]; then
                build_image
            elif [[ $job = 'integration_tests' ]]; then
                integration_tests
            elif [[ $job = 'k8s_integration_tests' ]]; then
                k8s_integration_tests
            fi
            ;;
        \? )
            echo "Invalid option: $OPTARG" 1>&2
            ;;
        : )
            echo "Invalid option: $OPTARG requires an argument" 1>&2
            ;;
  esac
done
shift $((OPTIND -1))
