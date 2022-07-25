####################
# WORK IN PROGRESS #
####################
# Module which should check if the VPC have the necessary tags.
# If all tags = COMPLIANT
# If missing one ore more tags = NOT_COMPLIANT

vpc_tags_to_check = {
    'key': 'value',
    'placeholder1': 'placeholder1',
    'placeholder2': 'placeholder2',
}

def vpc_tag_compliance_check(ec2_client, vpcId):
    vpc_tag_compliance={}
    assumed_role = assume_role(accountId)
    ec2 = assumed_role.client('ec2', region_name=region)
    vpc = ec2.describe_vpcs(VpcIds=[vpcId])['Vpcs'][0]
    try:
        vpc_tags = vpc['Tags']
        vpc_tags_dict = {tag["Key"]: tag["Value"] for tag in vpc_tags}
        missing_tags=[]
        existing_tags=[]
        for tag in vpc_tags_to_check.items():
            print('checking tag: '+tag[0]+':'+tag[1])
            if tag in vpc_tags_dict.items():
                print('Tag: '+tag[0]+':'+tag[1]+' exists on VPC!')
                existing_tags.append(tag[0]+':'+tag[1])
            else:
                print('Tag: '+tag[0]+':'+tag[1]+' DOES NOT exists on VPC!')
                missing_tags.append(tag[0]+':'+tag[1])
        if len(missing_tags) > 0:    
            vpc_tag_compliance['Status']='NOT_COMPLIENT'
            vpc_tag_compliance['Resources']=missing_tags
            vpc_tag_compliance['Comment']='missing tags: '+missing_tags
        else:
            vpc_tag_compliance['Status']='COMPLIENT'
            vpc_tag_compliance['Resources']=existing_tags
            vpc_tag_compliance['Comment']='VPC is correctly tagged'
    except:
        print('No tags found')
        vpc_tag_compliance['Status']='NOT_COMPLIENT'
        vpc_tag_compliance['Resources']='Null'
        vpc_tag_compliance['Comment']='No tags found'
    return vpc_tag_compliance
