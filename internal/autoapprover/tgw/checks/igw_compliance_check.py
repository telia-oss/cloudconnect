# Checking if the VPC have a Internet Gateway (IGW). 
# If IGW exist, the check warns (COMPLIENT_WARN) as sometimes a IGW is needed for some functionality such as: https://aws.amazon.com/blogs/networking-and-content-delivery/accessing-private-application-load-balancers-and-instances-through-aws-global-accelerator/ 
# where Global Accelerator can route to private resources. (Similar to AWS API GW can route to a VPC resource such as AWS Lambda and PrivateLinks).
# However, if a IGW exist it checks if there are any routes to IGW in the route tables which would mean Public Subnets (NOT_COMPLIANT).
# If no IGW exist the status is considered as COMPLIANT.
#
# When calling the module you ingest the ec2 client you want to use and the vpcId you want to check for IGWs in.
# Example:
# ec2_client = assumed_role.client('ec2', region_name=region)
# vpcId = vpc-xyz1234
# igw_compliance_results = igw_compliance_check(ec2_client, vpcId)

from utils.compliance_result_template import compliance_result_template

def igw_compliance_check(ec2_client, vpcId):
    igw = ec2_client.describe_internet_gateways( 
        Filters=[{'Name':'attachment.vpc-id','Values':[vpcId]}]
        )
    try:
        igw_id = igw['InternetGateways'][0]['InternetGatewayId']
        print('IGW found '+igw_id)
        ### Checking if we have any Routes towards IGW. If any routes toward IGW exist the VPC is not compliant (Public Subnets)
        igw_routes = ec2_client.describe_route_tables(
            Filters=[{'Name':'route.gateway-id','Values':[igw_id]}]
        )
        route_tables=[]
        for route_table in igw_routes['RouteTables']:
            route_tables.append(route_table['RouteTableId'])
        if len(route_tables) > 0:
            igw_compliance = compliance_result_template('NOT_COMPLIENT', igw_id+' & '+str(route_tables),'IGW route found in Route tables: '+str(route_tables))
        else:
            igw_compliance = compliance_result_template('COMPLIENT_WARN', igw_id,'IGW found but with no routes')
    except IndexError:
        igw_compliance = compliance_result_template('COMPLIENT', 'Null','No IGW found')
    return igw_compliance