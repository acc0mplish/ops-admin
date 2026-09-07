-- MySQL dump 10.13  Distrib 8.0.46, for Linux (x86_64)
--
-- Host: localhost    Database: ops_admin
-- ------------------------------------------------------
-- Server version	8.0.46

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `domain_public_dns_account`
--

DROP TABLE IF EXISTS `domain_public_dns_account`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `domain_public_dns_account` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  `provider` varchar(32) NOT NULL,
  `access_key_cipher` text NOT NULL,
  `secret_key_cipher` text NOT NULL,
  `status` bigint NOT NULL DEFAULT '1',
  `last_connection_status` varchar(32) DEFAULT NULL,
  `last_connection_error` varchar(500) DEFAULT NULL,
  `last_connection_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_domain_public_dns_account_provider` (`provider`),
  KEY `idx_domain_public_dns_account_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_certificates`
--

DROP TABLE IF EXISTS `ssl_certificates`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_certificates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(128) NOT NULL,
  `main_domain` varchar(255) NOT NULL,
  `type` varchar(16) NOT NULL,
  `source` varchar(16) NOT NULL,
  `provider` varchar(32) DEFAULT NULL,
  `dns_account_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL,
  `issuer` varchar(255) DEFAULT NULL,
  `serial_number` varchar(255) DEFAULT NULL,
  `fingerprint_sha256` varchar(128) DEFAULT NULL,
  `key_algorithm` varchar(64) DEFAULT NULL,
  `not_before` datetime(3) DEFAULT NULL,
  `not_after` datetime(3) DEFAULT NULL,
  `certificate_pem` longtext,
  `private_key_cipher` longtext,
  `certificate_chain` longtext,
  `auto_renew` tinyint(1) NOT NULL DEFAULT '0',
  `renew_before_days` bigint NOT NULL DEFAULT '30',
  `cloud_certificate_id` varchar(255) DEFAULT NULL,
  `cloud_sync_status` varchar(16) NOT NULL,
  `last_sync_at` datetime(3) DEFAULT NULL,
  `last_sync_error` varchar(1000) DEFAULT NULL,
  `last_renew_attempt` datetime(3) DEFAULT NULL,
  `last_renew_error` varchar(1000) DEFAULT NULL,
  `renew_retry_count` bigint NOT NULL DEFAULT '0',
  `acmeca` varchar(500) DEFAULT NULL,
  `acme_email` varchar(255) DEFAULT NULL,
  `include_root_domain` tinyint(1) NOT NULL DEFAULT '0',
  `has_private_key` tinyint(1) NOT NULL DEFAULT '0',
  `created_by` bigint unsigned DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_certificates_main_domain` (`main_domain`),
  KEY `idx_ssl_certificates_type` (`type`),
  KEY `idx_ssl_certificates_source` (`source`),
  KEY `idx_ssl_certificates_provider` (`provider`),
  KEY `idx_ssl_certificates_dns_account_id` (`dns_account_id`),
  KEY `idx_ssl_certificates_status` (`status`),
  KEY `idx_ssl_certificates_fingerprint_sha256` (`fingerprint_sha256`),
  KEY `idx_ssl_certificates_not_before` (`not_before`),
  KEY `idx_ssl_certificates_not_after` (`not_after`),
  KEY `idx_ssl_certificates_auto_renew` (`auto_renew`),
  KEY `idx_ssl_certificates_cloud_certificate_id` (`cloud_certificate_id`),
  KEY `idx_ssl_certificates_cloud_sync_status` (`cloud_sync_status`),
  KEY `idx_ssl_certificates_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ssl_certificate_versions`
--

DROP TABLE IF EXISTS `ssl_certificate_versions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ssl_certificate_versions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `certificate_id` bigint unsigned NOT NULL,
  `version` bigint NOT NULL,
  `certificate_pem` longtext NOT NULL,
  `private_key_cipher` longtext NOT NULL,
  `certificate_chain` longtext,
  `fingerprint_sha256` varchar(128) DEFAULT NULL,
  `not_before` datetime(3) DEFAULT NULL,
  `not_after` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ssl_certificate_versions_certificate_id` (`certificate_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ops_schedule_task`
--

DROP TABLE IF EXISTS `ops_schedule_task`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ops_schedule_task` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(128) NOT NULL,
  `task_type` varchar(32) NOT NULL,
  `template_id` bigint unsigned DEFAULT NULL,
  `script_id` bigint unsigned DEFAULT NULL,
  `script_name` varchar(128) DEFAULT NULL,
  `parameters` text,
  `variables` text,
  `host_ids_json` text,
  `group_ids_json` text,
  `concurrency` bigint DEFAULT '5',
  `http_method` varchar(16) DEFAULT NULL,
  `url` varchar(1024) DEFAULT NULL,
  `headers_json` text,
  `body` longtext,
  `expected_status` bigint DEFAULT '200',
  `timeout_seconds` bigint DEFAULT '10',
  `cron_expr` varchar(128) NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `status` bigint DEFAULT '1',
  `notify_enabled` tinyint(1) DEFAULT '0',
  `notify_rule_id` bigint unsigned DEFAULT NULL,
  `notify_on_failure_only` tinyint(1) DEFAULT '0',
  `last_status` varchar(32) DEFAULT NULL,
  `last_summary` text,
  `last_run_at` datetime(3) DEFAULT NULL,
  `next_run_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ops_schedule_task_name` (`name`),
  KEY `idx_ops_schedule_task_task_type` (`task_type`),
  KEY `idx_ops_schedule_task_template_id` (`template_id`),
  KEY `idx_ops_schedule_task_script_id` (`script_id`),
  KEY `idx_ops_schedule_task_status` (`status`),
  KEY `idx_ops_schedule_task_notify_enabled` (`notify_enabled`),
  KEY `idx_ops_schedule_task_notify_rule_id` (`notify_rule_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-09-06 15:34:00
