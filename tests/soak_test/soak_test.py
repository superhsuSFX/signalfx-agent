import os
import sys
import time
import yaml

import boto3
import paramiko
import pytest


class AWS:
    def __init__(self, aws_config):
        self.aws_config = aws_config
        self.pempath = "/tmp/" + self.aws_config["keypair"] + ".pem"
        self.instances_collection = []
        self.aws_session()
        self.create_instance()

    def aws_session(self):
        access_key, secret_access_key = self.get_aws_keys()
        self.session = boto3.Session(
            aws_access_key_id=access_key,
            aws_secret_access_key=secret_access_key,
            region_name=self.aws_config["region"],
        )

        self.resource = self.session.resource(self.aws_config["resource_type"])

    def create_instance(self):
        print("INFO Creating AWS instances")
        with open(self.pempath,"w") as outfile:
            self.key_pair = self.resource.create_key_pair(KeyName=self.aws_config["keypair"])
            key_pair_out = str(self.key_pair.key_material)
            outfile.write(key_pair_out)
        os.chmod(self.pempath, 0o400)
        print("INFO Pemfile location: {}".format(self.pempath))
        instances = self.resource.create_instances(
            ImageId=self.aws_config["image_id"],
            MinCount=self.aws_config["min"],
            MaxCount=self.aws_config["max"],
            KeyName=self.aws_config["keypair"],
            InstanceType=self.aws_config["instance_type"],
            BlockDeviceMappings=[
                {
                    "DeviceName": "/dev/sda1",
                    "Ebs": {
                        "DeleteOnTermination": True,
                        "VolumeSize": self.aws_config["volume_size"],
                        "VolumeType": "gp2",
                    },
                }
            ],
            TagSpecifications=[ 
                {
                    "ResourceType": "instance",
                    "Tags": [
                        {
                            "Key": "Name",
                            "Value": "signalfx-agent-test",
                        },
                    ]
                },
                {
                    "ResourceType": "volume",
                    "Tags": [
                        {
                            "Key": "Name",
                            "Value": "signalfx-agent-test",
                        },
                    ]
                },
            ]
        )
        for ins in instances:
            ins.wait_until_running()
        self.instance_ids = [ins.instance_id for ins in instances]

    def get_hostnames(self):
        print("INFO Fetching AWS instances hostnames")
        self.instances_collection = self.resource.instances.filter(
            Filters=[ {  "Name": "instance-id",    "Values": self.instance_ids } ]
        )
        hostnames = [ins.public_dns_name for ins in self.instances_collection]
        return hostnames

    def terminate_instances(self):
        print("INFO Terminating AWS instances (id's: {})".format(self.instance_ids))
        for ins in self.instances_collection:
            ins.terminate()
        print("INFO Removing key pair (id's: {})".format(self.key_pair))
        self.key_pair.delete()
        print("INFO Removing Pem file (path: {})".format(self.pempath))
        os.remove(self.pempath)

    def get_aws_keys(self):
        access_key = self.aws_config["access_key"]
        secret_access_key = self.aws_config["secret_access_key"]
        if access_key in ["$AWS_ACCESS_KEY_ID", "${AWS_ACCESS_KEY_ID}"]:
            access_key = os.environ['AWS_ACCESS_KEY_ID']
        if secret_access_key in ["$AWS_SECRET_ACCESS_KEY", "${AWS_SECRET_ACCESS_KEY}"]:
            secret_access_key = os.environ['AWS_SECRET_ACCESS_KEY']
        return access_key, secret_access_key

def do_filetransfer(ssh_handle, source, dest):
    """
    Transfer file onto specified target
    """
    ftp_client=ssh_handle.open_sftp()
    ftp_client.put(source, dest)
    ftp_client.close()
    print("INFO soak-addon.sh file transfer completed")

def exec_command_ssh(ssh_handle, command):
    """
    Execute SSH commands
    """
    print("INFO Executing `{}` command".format(command))
    stdin, stdout, stderr = ssh_handle.exec_command(command)
    stdin.flush()
    data = stdout.read().splitlines()
    for line in data:
        line_decoded = line.decode()
        print(line_decoded)
    data = stderr.read().splitlines()
    if data:
        print("STDERR OF SSH")
        for line in data:
            line_decoded = line.decode()
            print(line_decoded)

def create_ssh(host, username, pemfile):
    """
    Creates SSH communication connection
    """
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    privkey = paramiko.RSAKey.from_private_key_file(pemfile)
    ssh.connect(host,username=username,pkey=privkey)
    return ssh

def close_ssh(ssh_obj):
    """
    Close SSH communication connection
    """
    ssh_obj.close()

def connect_to_instance(config, hostnames, username, pemfile):
    """
    Connect to specified instance and executes jobs.
    """
    for host in hostnames:
        ssh = create_ssh(host, username, pemfile)
        addon_file = os.path.join(os.path.dirname(os.path.realpath(__file__)), "soak-addon.sh")
        do_filetransfer(ssh, addon_file,"soak-addon.sh")
        exec_command_ssh(ssh, "bash ~/soak-addon.sh -j install")
        exec_command_ssh(ssh, "bash ~/soak-addon.sh -j checkout")
        close_ssh(ssh)
        if config["jobs"]:
            ssh = create_ssh(host, username, pemfile)
            for job in config["jobs"]:
                exec_command_ssh(ssh, "bash ~/soak-addon.sh -j {}".format(job))
            close_ssh(ssh)

def create_setup(config):
    """
    Create environment for smartagent testing
    """
    cloud_provider = config["cloud_provider"]
    if "aws" in cloud_provider:
        aws_config = cloud_provider["aws"]
        aws_provider = AWS(aws_config)
        hostnames = aws_provider.get_hostnames()
        print("INFO Created Instances {}, and waiting for instance to come up".format(aws_provider.instance_ids))
        time.sleep(config["instance_wait_time"])
        connect_to_instance(config, hostnames, aws_config["username"], aws_provider.pempath)
        if aws_config["terminate"]:
            aws_provider.terminate_instances()
    else:
        print("Unknown cloud provider {} in soak-config, exiting.".format(cloud_provider.keys()))
        sys.exit()

def process_config():
    """
    Process soak test configuration
    """
    yaml_config = os.path.join(os.path.dirname(os.path.realpath(__file__)), "soak-config.yaml")
    with open(yaml_config, "r") as stream:
        soak_config = yaml.load(stream)
    return soak_config

@pytest.mark.soak
def test_soak():
    """
    Run soak test
    """
    start = time.time()
    config = process_config()
    create_setup(config)
    print("INFO Total time taken: %f minutes" % ((time.time()-start)/60))


