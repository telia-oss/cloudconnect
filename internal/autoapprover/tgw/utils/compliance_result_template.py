# Builds the compliance report which technically is a python dictionary.
# The report is a "Dictionary in Dictionary" and looks like the following:
#{
#'check1': {'Status': '<Complient Status1>', 'Resource': '<Resource Id1>', 'Comment': '<Comment1>'},
#'check2': {'Status': '<Complient Status2>', 'Resource': '<Resource Id2>', 'Comment': '<Comment2>'},
# }
#
# You build this report by calling the module with an argument for each key of the report.
# To build the following report:
# {'VPC Compliance': {'Status': 'NOT_COMPLIENT', 'Resource': '<vpcId>', 'Comment': <vpcId>+'is Default VPC'}}
#
# You run the following:
# my_compliance_report = compliance_result_template('NOT_COMPLIENT', resource_id, vpcId+'is Default VPC')
# compliance_report['VPC Compliance']=my_compliance_report


def compliance_result_template(status, resource, comment):
    compliance_result={}
    compliance_result['Status']=status
    compliance_result['Resource']=resource
    compliance_result['Comment']=comment
    return compliance_result
