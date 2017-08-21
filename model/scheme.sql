CREATE DATABASE IF NOT EXISTS `pixty`
	DEFAULT CHARACTER SET utf8
	DEFAULT COLLATE utf8_bin;

USE pixty;

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
	UNIQUE `id_idx` USING BTREE (id)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `field_info` (
	`org_id`                 BIGINT(20) NOT NULL,
	`field_id`               BIGINT(20) NOT NULL,
	`field_type`             VARCHAR(50) NOT NULL,
	`display_name`           VARCHAR(255) NOT NULL,
	UNIQUE `org_id_field_id_idx` USING BTREE (org_id, field_id)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `user` (
	`id`                     BIGINT(20)      NOT NULL AUTO_INCREMENT,
	`login`                  VARCHAR(255)    NOT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	UNIQUE `login_idx` USING BTREE (login)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `camera` (
	`id`                    VARCHAR(255) NOT NULL,
	`org_id`                BIGINT(20) NOT NULL,
	`secret_key`            VARCHAR(50),
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `org_id_idx` USING BTREE (org_id),
	UNIQUE `access_key_idx` USING BTREE (access_key)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

CREATE TABLE IF NOT EXISTS `person` (
	`id`                  VARCHAR(255) NOT NULL,
	`cam_id`              VARCHAR(255) NOT NULL,
	`last_seen`           BIGINT(20)      NOT NULL,
	`profile_id`          BIGINT(20)      NOT NULL,
	`picture_id`          VARCHAR(255) NOT NULL,
	`match_group`         VARCHAR(255) NOT NULL,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `cam_id_idx` USING BTREE (cam_id),
	INDEX `last_seen_idx` USING BTREE (last_seen),
	INDEX `profile_id_idx` USING BTREE (profile_id),
	INDEX `match_group_idx` USING BTREE (match_group)
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
	INDEX `captured_at_idx` USING BTREE (captured_at)
) ENGINE=`InnoDB` AUTO_INCREMENT=1 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;


CREATE TABLE IF NOT EXISTS `face` (
	`id`                     BIGINT(20)      NOT NULL AUTO_INCREMENT,
	`person_id`              VARCHAR(255)    NOT NULL,
	`captured_at`			  BIGINT(20)      NOT NULL,
	`image_id`               VARCHAR(255)    NOT NULL,
	`img_top`                INT,
	`img_left`               INT,
	`img_bottom`             INT,
	`img_right`              INT,
	`face_image_id`          VARCHAR(255)    NOT NULL,
	`v128d`                  BLOB,
	PRIMARY KEY (`id`),
	UNIQUE `id_idx` USING BTREE (id),
	INDEX `person_id_idx` USING BTREE (person_id),
	INDEX `captured_at_idx` USING BTREE (captured_at)
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
	`value`                      VARCHAR(16535)  NOT NULL, 
	UNIQUE `profile_id_field_id_idx` USING BTREE (profile_id, field_id)
) ENGINE=`InnoDB` DEFAULT CHARACTER SET utf8 COLLATE utf8_bin ROW_FORMAT=COMPACT CHECKSUM=0 DELAY_KEY_WRITE=0;

#After creation for test camera
#insert into organization(id, name) values(1, 'pixty');
#insert into camera(id, org_id, secret_key) values("pixtyTestCam", 1, "1234");
