import boto3
#import pandas as pd
from utils.assume_role import assume_role

#import checks
from checks.igw_compliance_check import igw_compliance_check
from checks.default_vpc_compliance_check import default_vpc_compliance_check
from checks.ipam_compliance_check import ipam_compliance_check
from checks.ipam_cidr_compliance_check import ipam_cidr_compliance_check

ec2 = boto3.client('ec2')

def report(accountId, region, vpcId):
    assumed_role = assume_role(accountId) #Assuming AutoApprover role in spoke account
    ec2_client = assumed_role.client('ec2', region_name=region) #configuring the EC2 API to use the AutoApprove role in spoke account
    compliance_report = {}
    #Start Default VPC compliance check
    default_vpc_compliance_results = default_vpc_compliance_check(ec2_client, vpcId)
    compliance_report['Default VPC Compliance']=default_vpc_compliance_results
    #Start IGW compliance check
    igw_compliance_results = igw_compliance_check(ec2_client, vpcId)
    compliance_report['IGW Compliance']=igw_compliance_results
    #Start IPAM compliance check
    ipam_compliance_results = ipam_compliance_check(vpcId)
    compliance_report['IPAM Compliance']=ipam_compliance_results
    #Start IPAM CIDR check 
    ipam_cidr_compliance_results = ipam_cidr_compliance_check(vpcId)
    compliance_report['IPAM CIDR Compliance']=ipam_cidr_compliance_results
    #Returns the report containing all checks
    return compliance_report

def lambda_handler(event, lambda_context):
    attachments_paginator = ec2.get_paginator('describe_transit_gateway_vpc_attachments')
    describe_transit_gateway_vpc_attachments_iterator = attachments_paginator.paginate(
        Filters=[{'Name':'state','Values':['pendingAcceptance']}])
    for attachments in describe_transit_gateway_vpc_attachments_iterator:
        a = attachments['TransitGatewayVpcAttachments']
        for attachment in a:
            accountId=attachment['VpcOwnerId']
            region='eu-west-1'
            vpcId=attachment['VpcId']
            tgwAttachmentId=attachment['TransitGatewayAttachmentId']
            print('\nWorking with: '+tgwAttachmentId+' from VPC: '+vpcId+' in account: '+accountId)
            compliance_report = report(accountId, region, vpcId)
            #Checks if there are any NON_COMPLIENT resources in the report
            report_status = [check["Status"] for check in compliance_report.values()]
            if 'NOT_COMPLIENT' in report_status:
                attachment_compliance='NOT_COMPLIENT'
            else:
                attachment_compliance='COMPLIENT'
            #Prints the attachment compliance report (dictionary).
            print('Attachment compliance: '+attachment_compliance)
            print(compliance_report)
            #Builds a table using pandas to be able to provide a human-readable report to users.
    #        df = pd.DataFrame(compliance_report).T
    #        pd.set_option("max_colwidth", 100)
    #        pd.set_option("display.colheader_justify","center")
    #        print(df)
