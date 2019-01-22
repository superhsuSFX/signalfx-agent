import argparse
import boto3
from functools import partial as p
import os
import paramiko
import sys
import time

class ParseArgs:
    def __init__(self):
        self.description = "signalfx-agent soak testing framework"

    def get_env(self, env):
        return os.getenv(env)

    def parse_args(self):
        parser = argparse.ArgumentParser(
            formatter_class=argparse.RawDescriptionHelpFormatter,
            description=self.description)
        parser.add_argument('-k', '--key-name', help='Security key name', 
                            type=str, required=False, dest='key_name', default='TestKey')
        parser.add_argument('-c', '--cloud-provider', help='Cloud Provider name', 
                            type=str, required=False, dest='cloud_provider', default='AWS')
        parser.add_argument('-a', '--access-key', help='AWS access key', 
                            type=str, required=False, dest='aws_access_key', 
                            default=self.get_env("AWS_ACCESS_KEY_ID"))
        parser.add_argument('-s', '--secret-access-key', help='AWS Secret access key', 
                            type=str, required=False, dest='aws_secret_access_key', 
                            default=self.get_env("AWS_SECRET_ACCESS_KEY"))
        parser.add_argument('-r', '--region', help='AWS Region', 
                            type=str, required=False, dest='aws_region', 
                            default=self.get_env("AWS_REGION"))
        parser.add_argument('-d', '--debug', help='Debug mode', 
                            type=bool, required=False, default='DEBUG')
        return parser.parse_args()

class AWS:
    def __init__(self, keyname):
        self.pem_ext = '.pem'
        self.aws_boto = boto3
        self.keyname = keyname
        self.pemfile = self.keyname + self.pem_ext

    def aws_session(self, access_key, secret_access_key, region):
        session = self.aws_boto.Session(aws_access_key_id=access_key,
                                aws_secret_access_key=secret_access_key,
                                region_name=region)
        return session

    def aws_resource(self, name, session):
        resource = session.resource(name)
        return resource

    def create_instance(self, resource, image_id, instance_type, min=1, max=1, volumesize=20):
        print("INFO Creating AWS instances")
        with open(self.pemfile,'w') as outfile:
            key_pair = resource.create_key_pair(KeyName=self.keyname)
            KeyPairOut = str(key_pair.key_material)
            outfile.write(KeyPairOut)
        os.chmod(self.pemfile, 0o400)
        instances = resource.create_instances(
            ImageId=image_id, 
            MinCount=min, 
            MaxCount=max,
            KeyName=self.keyname,
            InstanceType=instance_type,
            BlockDeviceMappings=[
                {
                    'DeviceName': '/dev/sda1',
                    'Ebs': {
                        'DeleteOnTermination': True,
                        'VolumeSize': volumesize,
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
    ftp_client=ssh_handle.open_sftp()
    ftp_client.put(source, dest)
    ftp_client.close()
    print("INFO soak-addon.sh file transfer completed")

def exec_command_ssh(ssh_handle, command):
    print("INFO Executing `{}` command".format(command))
    stdin, stdout, stderr = ssh_handle.exec_command(command)
    stdin.flush()
    data = stdout.read().splitlines()
    for line in data:
        x = line.decode()
        print(x)
    print("STDERR OF SSH")
    data = stderr.read().splitlines()
    for line in data:
        x = line.decode()
        print(x)


def connect_to_instance(hostnames, pemfile):
    for host in hostnames:
        ssh = paramiko.SSHClient()
        ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        privkey = paramiko.RSAKey.from_private_key_file(pemfile)
        ssh.connect(host,username='ubuntu',pkey=privkey)
        do_filetransfer(ssh, 'scripts/soak-addon.sh','soak-addon.sh')
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j checkout')
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j install')
        ssh.close()
        ssh.connect(host,username='ubuntu',pkey=privkey)
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j build')
        exec_command_ssh(ssh, 'bash ~/soak-addon.sh -j k8s_integration_tests')
        ssh.close()

def create_setup(args):
    cloud_provider = args.cloud_provider
    aws_access_key = args.aws_access_key
    aws_secret_access_key = args.aws_secret_access_key
    aws_region = args.aws_region
    if cloud_provider.lower() == 'aws':
        aws_provider = AWS(args.key_name)
        session = aws_provider.aws_session(aws_access_key, aws_secret_access_key, aws_region)
        resource = aws_provider.aws_resource('ec2', session)
        instances = aws_provider.create_instance(resource, 'ami-0ea790e761025f9ce', 't2.large', volumesize=50)
        hostnames = aws_provider.get_hostnames(resource, instances)
        print("INFO Created Instances {}, and waiting for isntance to come up".format(instances))
        time.sleep(180)
        connect_to_instance(hostnames, aws_provider.pemfile)
    else:
        print("Unknown cloud provider {}, exiting.".format(cloud_provider))
        sys.exit()

def main():
    start = time.time()
    args_cls = ParseArgs()
    args = args_cls.parse_args()
    machines = create_setup(args)
    print('INFO Total time taken: %f minutes' % ((time.time()-start)/60))

if __name__ == '__main__':
    main()

