# Assumes the AutoApprover role in other accounts (spoke accounts).
# The AutoApprover role in the spoke accounts needs to have a trust policy that allows
# the Lambda to assume the AutoApprove role.
#
# Use the AccountId you want to assume into as the Argument.

import boto3
sts_connection = boto3.client('sts')

def assume_role(accountId):
    spoke_account = sts_connection.assume_role(
        RoleArn='arn:aws:iam::'+accountId+':role/AutoApprover',
        RoleSessionName='AutoApproverLambda'
    )
    return boto3.Session(
        aws_access_key_id=spoke_account['Credentials']['AccessKeyId'],
        aws_secret_access_key=spoke_account['Credentials']['SecretAccessKey'],
        aws_session_token=spoke_account['Credentials']['SessionToken'],
        )
