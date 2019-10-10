
CREATE SEQUENCE serial MINVALUE 00101;


INSERT INTO Channels(Channels_id,Channel, description, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'Email', 'This will be channel for notifying client using emailin', now(),now(),now(),0);

INSERT INTO languages(language_id,name, description,region, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'English', 'This will be langusge for notifying client using emailing in english',England, now(),now(),now(),0);

INSERT INTO message_templetes(message_templete_id,templete_name,templete_value,language, applied_at,created_at,modified_at,version) VALUES 
	(nextval('serial'),'Receveid_templete', 'Username;Account;Amount;','English', now(),now(),now(),0);

	