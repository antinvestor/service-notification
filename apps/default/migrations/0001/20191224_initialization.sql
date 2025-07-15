

INSERT INTO routes(id, tenant_id, partition_id, name, description, mode, route_type, uri, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq1g', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg', 'Profile Email', 'Channel for emailing profile queries', 'trx', 'email', 'mem://default_email',  '2020-01-08 21:22:30',  '2020-01-08 21:22:30'),
	('9bsv0s23l8og00vgjq7g', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg', 'Profile Sms', 'Channel to sms profile queries', 'trx', 'sms', 'mem://default_sms',  '2020-01-08 21:22:30',  '2020-01-08 21:22:30');


-- Languages :
INSERT INTO languages(id, tenant_id, partition_id, name, code, description, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjqa0', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg','English', 'en', 'The default language on the platform', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');



-- Template : default messages to allow a profile be registered

INSERT INTO templates(id, tenant_id, partition_id, name, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq90', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg', 'template.profilev1.contact.verification', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');

INSERT INTO template_data(id, tenant_id, partition_id,  template_id, language_id, type, detail, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq90', 'c2f4j7au6s7f91uqnojg', 'c2f4j7au6s7f91uqnokg', '9bsv0s23l8og00vgjq90', '9bsv0s23l8og00vgjqa0', 'text', 'Your contact verification code is : {{.pin}} and will expire at {{.expiryDate}}', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');
