

INSERT INTO channels(id, product_id, name, description, mode, type, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq1g', '#', 'Profile Email', 'Channel for emailing profile queries', 'trx', 'email',  '2020-01-08 21:22:30',  '2020-01-08 21:22:30'),
	('9bsv0s23l8og00vgjq7g', '#', 'Profile Sms', 'Channel to sms profile queries', 'trx', 'sms',  '2020-01-08 21:22:30',  '2020-01-08 21:22:30');


-- Languages :
INSERT INTO languages(id, name, code, description, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjqa0','English', 'en', 'The default language on the platform', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');



-- Template : default messages to allow a profile be registered

INSERT INTO templetes(id, product_id, language_id, name, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq90', '#', '9bsv0s23l8og00vgjqa0', 'template.papi.contact.verification', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');

INSERT INTO templete_data(id, templete_id, type, detail, created_at, modified_at) VALUES
	('9bsv0s23l8og00vgjq90', '9bsv0s23l8og00vgjq90', 'text', 'Your contact verification code is : {{.pin}} and will expire at {{.expiryDate}}', '2020-01-08 21:22:30',  '2020-01-08 21:22:30');
