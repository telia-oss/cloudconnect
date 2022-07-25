# Check if the VPC is a default VPC.
# If Default VPC, the VPC is considered as NOT_COMPLIANT
# If not a Default VPC, the VPC is considered as COMPLIANT
#
# When calling the module you ingest the ec2 client you want to use and the vpcId you want to check for IGWs in.
# Example:
# ec2_client = assumed_role.client('ec2', region_name=region)
# vpcId = vpc-xyz1234
# default_vpc_compliance_results = default_vpc_compliance_check(ec2_client, vpcId)

from utils.compliance_result_template import compliance_result_template

def default_vpc_compliance_check(ec2_spoke, vpcId):
    vpc = ec2_spoke.describe_vpcs(VpcIds=[vpcId])['Vpcs'][0]
    ### Start Default VPC check - If default VPC. If Default VPC the VPC is not compliant.
    if vpc['IsDefault'] == 'True':
        default_vpc_compliance = compliance_result_template('NOT_COMPLIENT', vpcId, vpcId+' is Default VPC')
    else:
        default_vpc_compliance = compliance_result_template('COMPLIENT', vpcId, vpcId+' is not Default VPC')
    return  default_vpc_compliance