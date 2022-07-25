# Checks if the VPC is IPAM compliant.
# The IPAM compliance check is actually built in into AWS IPAM and this check is just re-using the
# native functionality which checks if the allocation is compliant with the allocation rules of the IPAM pool.
# For more info, check "Compliance status" in: https://docs.aws.amazon.com/vpc/latest/ipam/monitor-cidr-compliance-ipam.html
#
# When calling the module you use the vpcId you want to check for IPAm compliance as argument.
# Example:
# vpcId = vpc-xyz1234
# ipam_compliance_results = ipam_compliance_check(vpcId)
import boto3
from utils.compliance_result_template import compliance_result_template

ipamScope='ipam-scope-077847d1e5c437ed7'
ipam_region='eu-west-1'

def ipam_compliance_check(vpcId):
    ec2 = boto3.client('ec2', region_name=ipam_region)
    cidrs = ec2.get_ipam_resource_cidrs(ResourceId=vpcId,IpamScopeId=ipamScope)['IpamResourceCidrs']
    for cidr in cidrs:
        if cidr['ComplianceStatus'] != 'compliant':
            ipam_compliance = compliance_result_template('NOT_COMPLIENT', vpcId+' & '+ipamScope, 'IPAM status is not compliant in IPAM, check reason with network team')
        else:
            ipam_compliance = compliance_result_template('COMPLIENT', vpcId, 'IPAM status is compliant')
    return ipam_compliance
