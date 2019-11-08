CREATE SEQUENCE templetes_serial MINVALUE 0010000;

INSERT INTO message_templetes(message_templete_id,templete_name,templete_value,language_id, applied_at,created_at,modified_at,version) VALUES 
	(nextval('templetes_serial'),'Receveid_templete', 'Username;Account;Amount;','lg_23d5f667hhn', now(),now(),now(),0);
