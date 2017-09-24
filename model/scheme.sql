CREATE DATABASE IF NOT EXISTS `pixty_test`
	DEFAULT CHARACTER SET utf8
	DEFAULT COLLATE utf8_bin;

USE pixty_test;

#DROP TABLE IF EXISTS `organization`;
#DROP TABLE IF EXISTS `field_info`;
#DROP TABLE IF EXISTS `user`;
#DROP TABLE IF EXISTS `camera`;
#DROP TABLE IF EXISTS `person`;
#DROP TABLE IF EXISTS `face`;
#DROP TABLE IF EXISTS `profile`;
#DROP TABLE IF EXISTS `profile_meta`;

CREATE TABLE IF NOT EXISTS `organization` (
	`id`                     BIGINT(20)       NOT NULL AUTO_INCREMENT,
	`name`                   VARCHAR(255)     DEFAULT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	UNIQUE `name_idx` USING BTREE (name)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_unicode_ci ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `user` (
	`id`                     BIGINT(20)     NOT NULL AUTO_INCREMENT,
	`login`                  VARCHAR(50)    NOT NULL,
	`email`                  VARCHAR(50)    NOT NULL,
	`salt`                   VARCHAR(128)   NOT NULL,
	`hash`                   VARCHAR(255)   NOT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	UNIQUE `login_idx` USING BTREE (login),
	INDEX `email_idx` USING BTREE (email)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_unicode_ci ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `user_role` (
	`login`                  VARCHAR(50)    NOT NULL,
	`org_id`                 BIGINT(20)     NOT NULL,
	`role`                   VARCHAR(128)   NOT NULL,
	INDEX `login_idx` USING BTREE (login),
	FOREIGN KEY (`login`) REFERENCES user(`login`) ON DELETE CASCADE,
	UNIQUE `org_login_idx` USING BTREE (org_id, login)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_unicode_ci ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `camera` (
	`id`                    BIGINT(20) NOT NULL AUTO_INCREMENT,
	`name`                  VARCHAR(255) NOT NULL,
	`org_id`                BIGINT(20) NOT NULL,
	`secret_key`            VARCHAR(50),
	PRIMARY KEY (`id`),
	UNIQUE `name_org_idx` USING BTREE (name, org_id),
	INDEX `org_id_idx` USING BTREE (org_id)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

#Field Info. Please pay attention that display_name is case INSENSITIVE 'aaa' == 'AaA'
CREATE TABLE IF NOT EXISTS `field_info` (
	`id`                     BIGINT(20)       NOT NULL AUTO_INCREMENT,
	`org_id`                 BIGINT(20) NOT NULL,
	`field_type`             VARCHAR(50) NOT NULL,
	`display_name`           VARCHAR(255) NOT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	UNIQUE `org_id_display_name_idx` USING BTREE (org_id, display_name)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_unicode_ci ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `picture` (
	`id`                  VARCHAR(255) NOT NULL,
	`refs`                INT NOT NULL DEFAULT 0,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `refs_idx` USING BTREE (refs)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `person` (
	`id`                  VARCHAR(255) NOT NULL,
	`cam_id`              BIGINT(20)      NOT NULL,
	`created_at`          BIGINT(20)      NOT NULL,
	`last_seen`           BIGINT(20)      NOT NULL,
	`profile_id`          BIGINT(20)      NOT NULL,
	`picture_id`          VARCHAR(255) NOT NULL,
	`match_group`         BIGINT(20) NOT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `cam_id_idx` USING BTREE (cam_id),
	INDEX `created_at_idx` USING BTREE (created_at),
	INDEX `last_seen_idx` USING BTREE (last_seen),
	INDEX `profile_id_idx` USING BTREE (profile_id),
	INDEX `match_group_idx` USING BTREE (match_group),
	FOREIGN KEY (`cam_id`) REFERENCES camera(id) ON DELETE RESTRICT
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `face` (
	`id`                    BIGINT(20)      NOT NULL AUTO_INCREMENT,
	`scene_id`              VARCHAR(255)    NOT NULL,
	`person_id`             VARCHAR(255)    NOT NULL,
	`captured_at`           BIGINT(20)      NOT NULL,
	`image_id`              VARCHAR(255)    NOT NULL,
	`img_left`              INT,
	`img_top`               INT,
	`img_right`             INT,
	`img_bottom`            INT,
	`face_image_id`         VARCHAR(255)    NOT NULL,
	`v128d`	                BLOB,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `person_id_idx` USING BTREE (person_id),
	INDEX `captured_at_idx` USING BTREE (captured_at),
	FOREIGN KEY (`person_id`) REFERENCES person(id) ON DELETE RESTRICT
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `profile` (
	`id`                     BIGINT(20)      NOT NULL AUTO_INCREMENT,
	`org_id`                 BIGINT(20)      NOT NULL,
	`picture_id`             VARCHAR(255)     NOT NULL, 
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `org_id_idx` USING BTREE (org_id)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `profile_meta` (
	`profile_id`                 BIGINT(20)      NOT NULL,
	`field_id`                   BIGINT(20)      NOT NULL,
	`value`                      VARCHAR(16000)  NOT NULL, 
	UNIQUE `profile_id_field_id_idx` USING BTREE (profile_id, field_id),
	FOREIGN KEY (`field_id`) REFERENCES field_info(id) ON DELETE CASCADE,
	FOREIGN KEY (`profile_id`) REFERENCES profile(id) ON DELETE CASCADE	
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `profile_kvs` (
	`profile_id`                 BIGINT(20)      NOT NULL,
	`key`                        VARCHAR(100)    NOT NULL,
	`value`                      VARCHAR(16000)  NOT NULL, 
	UNIQUE `profile_id_key_idx` USING BTREE (profile_id, `key`),
	FOREIGN KEY (`profile_id`) REFERENCES profile(id) ON DELETE CASCADE	
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;


# Triggers & procedures
delimiter |

CREATE PROCEDURE proc_inc_picture_ref(IN picture_id VARCHAR(255))
inc_picture_ref_exit:BEGIN
  IF picture_id IS NULL OR LENGTH(picture_id) = 0 THEN
    LEAVE inc_picture_ref_exit;
  END IF;
  IF (SELECT id FROM picture WHERE id=picture_id) IS NULL THEN
    INSERT INTO picture(id, refs) VALUES (picture_id, 1);
  ELSE
    UPDATE picture SET refs=refs+1 WHERE id=picture_id;
  END IF;
END;
|
CREATE PROCEDURE proc_dec_picture_ref(IN picture_id VARCHAR(255))
BEGIN
  UPDATE picture SET refs=refs-1 WHERE id=picture_id;
END;
|
CREATE TRIGGER trgr_new_face AFTER INSERT ON face
FOR EACH ROW BEGIN
  CALL proc_inc_picture_ref(NEW.image_id);
  CALL proc_inc_picture_ref(NEW.face_image_id);
END;
|
CREATE TRIGGER trgr_del_face BEFORE DELETE ON face
FOR EACH ROW BEGIN
  CALL proc_dec_picture_ref(OLD.image_id);
  CALL proc_dec_picture_ref(OLD.face_image_id);
END;
| 
CREATE TRIGGER trgr_new_person AFTER INSERT ON person
FOR EACH ROW BEGIN
  CALL proc_inc_picture_ref(NEW.picture_id);
END;
|
CREATE TRIGGER trgr_update_person AFTER UPDATE ON person
FOR EACH ROW BEGIN
  IF OLD.picture_id <> NEW.picture_id THEN
    CALL proc_inc_picture_ref(NEW.picture_id);
	CALL proc_dec_picture_ref(OLD.picture_id);
  END IF;
END;
|
CREATE TRIGGER trgr_del_person BEFORE DELETE ON person
FOR EACH ROW BEGIN
  CALL proc_dec_picture_ref(OLD.picture_id);
END;
| 
CREATE TRIGGER trgr_new_profile AFTER INSERT ON profile
FOR EACH ROW BEGIN
  CALL proc_inc_picture_ref(NEW.picture_id);
END;
|
CREATE TRIGGER trgr_update_profile AFTER UPDATE ON profile
FOR EACH ROW BEGIN
  IF OLD.picture_id <> NEW.picture_id THEN
    CALL proc_inc_picture_ref(NEW.picture_id);
	CALL proc_dec_picture_ref(OLD.picture_id);
  END IF;
END;
|
CREATE TRIGGER trgr_del_profile BEFORE DELETE ON profile
FOR EACH ROW BEGIN
  CALL proc_dec_picture_ref(OLD.picture_id);
END;
|
delimiter ;


DROP PROCEDURE IF EXISTS proc_inc_picture_ref;
DROP PROCEDURE IF EXISTS proc_dec_picture_ref;
DROP TRIGGER trgr_new_face;
DROP TRIGGER trgr_del_face;
DROP TRIGGER trgr_new_person;
DROP TRIGGER trgr_update_person;
DROP TRIGGER trgr_del_person;
DROP TRIGGER trgr_new_profile;
DROP TRIGGER trgr_update_profile;
DROP TRIGGER trgr_del_profile;


#After creation for test camera
#insert into organization(id, name) values(1, 'pixty');
#insert into camera(id, org_id, secret_key) values("ptt", 1, "1234");