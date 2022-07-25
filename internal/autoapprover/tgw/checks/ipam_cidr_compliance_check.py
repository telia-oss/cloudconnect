# Checks if the VPC CIDR is overallaping.
# The CIDR overlapping check is actually built in into AWS IPAM and this check is just re-using the
# native functionality which checks if the allocation with another VPC managed by IPAM.
# For more info, check "Overlap status" in: https://docs.aws.amazon.com/vpc/latest/ipam/monitor-cidr-compliance-ipam.html
#
# When calling the module you use the vpcId you want to check for overlap status as argument.
# Example:
# vpcId = vpc-xyz1234
# ipam_cidr_compliance_results = ipam_cidr_compliance_check(vpcId)
import boto3
from utils.compliance_result_template import compliance_result_template

ipamScope='ipam-scope-077847d1e5c437ed7'
ipam_region='eu-west-1'

def ipam_cidr_compliance_check(vpcId):
    ec2 = boto3.client('ec2', region_name=ipam_region)
    cidrs = ec2.get_ipam_resource_cidrs(ResourceId=vpcId,IpamScopeId=ipamScope)['IpamResourceCidrs']
    for cidr in cidrs:
        if cidr['OverlapStatus'] == 'overlapping':
            ipam_cidr_compliance = compliance_result_template('NOT_COMPLIENT', vpcId+' & '+ipamScope, 'VPC CIDR overlapps with another VPC!')
        else:
            ipam_cidr_compliance = compliance_result_template('COMPLIENT', vpcId+' & '+ipamScope, 'VPC CIDR is compliant')
    return ipam_cidr_compliance
