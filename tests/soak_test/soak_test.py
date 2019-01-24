import argparse
import boto3
from functools import partial as p
import os
import paramiko
import pytest
import sys
import time
import yaml

class AWS:
    def __init__(self, aws_config):
        self.pem_ext = '.pem'
        self.aws_config = aws_config
        self.keyname = aws_config['keypair']
        self.pemfile = self.keyname + self.pem_ext
        self.pempath = "/tmp/" + self.pemfile
        self.aws_boto = boto3
        self.session = self.aws_session()

    def aws_session(self):
        session = self.aws_boto.Session(aws_access_key_id=self.aws_config['access_key'],
                                aws_secret_access_key=self.aws_config['secret_access_key'],
                                region_name=self.aws_config['region'])
        return session

    def aws_resource(self, name):
        resource = self.session.resource(name)
        return resource

    def create_instance(self, resource):
        print("INFO Creating AWS instances")
        with open(self.pempath,'w') as outfile:
            key_pair = resource.create_key_pair(KeyName=self.keyname)
            KeyPairOut = str(key_pair.key_material)
            outfile.write(KeyPairOut)
        os.chmod(self.pempath, 0o400)
        print("INFO Pemfile location: {}".format(self.pempath))
        instances = resource.create_instances(
            ImageId=self.aws_config['image_id'], 
            MinCount=self.aws_config['min'], 
            MaxCount=self.aws_config['max'],
            KeyName=self.keyname,
            InstanceType=self.aws_config['instance_type'],
            BlockDeviceMappings=[
                {
                    'DeviceName': '/dev/sda1',
                    'Ebs': {
                        'DeleteOnTermination': True,
                        'VolumeSize': self.aws_config['volume_size'],
                        'VolumeType': 'gp2',
                    },
                },
            ],
            TagSpecifications=[ 
                {
                    'ResourceType': 'instance',
                    'Tags': [
                        {
                            'Key': 'Name',
                            'Value': 'signalfx-agent-test2'
                        },
                    ]
                },
                {
                    'ResourceType': 'volume',
                    'Tags': [
                        {
                            'Key': 'Name',
                            'Value': 'signalfx-agent-test2'
                        },
                    ]
                },
            ]
        )
        for ins in instances:
            ins.wait_until_running()
        return instances

    def get_hostnames(self, resource, instances):
        print("INFO Fetching AWS instances hostnames")
        instances_data = resource.instances.filter(Filters=[ {  'Name': 'instance-id',    'Values': [ins.instance_id for ins in instances] } ] )
        hostnames = [ins.public_dns_name for ins in instances_data]
        return hostnames

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
        x = line.decode()
        print(x)
    data = stderr.read().splitlines()
    if data:
        print("STDERR OF SSH")
        for line in data:
            x = line.decode()
            print(x)

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
        do_filetransfer(ssh, addon_file,'soak-addon.sh')
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j install')
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j checkout')
        close_ssh(ssh)
        ssh = create_ssh(host, username, pemfile)
        for job in config['jobs']:
            exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j {}'.format(job))
        close_ssh(ssh)

def create_setup(config):
    """
    Create environment for smartagent testing
    """
    cloud_provider = config['cloud_provider']
    if 'aws' in cloud_provider:
        aws_config = cloud_provider['aws']
        aws_provider = AWS(aws_config)
        resource = aws_provider.aws_resource(aws_config['resource_type'])
        instances = aws_provider.create_instance(resource)
        hostnames = aws_provider.get_hostnames(resource, instances)
        print("INFO Created Instances {}, and waiting for isntance to come up".format(instances))
        time.sleep(config['instance_wait_time'])
        connect_to_instance(config, hostnames, aws_config['username'], aws_provider.pempath)
    else:
        print("Unknown cloud provider {} in soak-config, exiting.".format(cloud_provider.keys()))
        sys.exit()

def process_config():
    """
    Process soak test configuration
    """
    yaml_config = os.path.join(os.path.dirname(os.path.realpath(__file__)), "soak-config.yaml")
    with open(yaml_config, 'r') as stream:
        soak_config = yaml.load(stream)
    return soak_config

@pytest.mark.soak
def test_soak():
    """
    Run soak test
    """
    start = time.time()
    config = process_config()
    machines = create_setup(config)
    print('INFO Total time taken: %f minutes' % ((time.time()-start)/60))


