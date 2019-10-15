CREATE SEQUENCE templetes_serial MINVALUE 0010000;

INSERT INTO message_templetes(message_templete_id,templete_name,templete_value,language, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'Receveid_templete', 'Username;Account;Amount;','English', now(),now(),now(),0);
