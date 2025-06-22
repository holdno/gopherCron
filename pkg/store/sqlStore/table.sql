CREATE TABLE `gc_org` (
  `id` varchar(32) NOT NULL COMMENT '组织id',
  `title` varchar(32) NOT NULL COMMENT '组织名称',
  `create_time` bigint(20) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_org_relevance` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `uid` bigint(20) NOT NULL COMMENT '关联用户id',
  `oid` varchar(32) NOT NULL COMMENT '关联组织id',
  `role` varchar(32) NOT NULL DEFAULT 'user' COMMENT '项目角色',
  `create_time` bigint(20) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `uid` (`uid`),
  KEY `oid` (`oid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_project` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `uid` bigint(20) NOT NULL COMMENT '关联用户id',
  `title` varchar(100) NOT NULL COMMENT '项目名称',
  `remark` varchar(255) NOT NULL COMMENT '项目备注',
  `token` varchar(32) NOT NULL DEFAULT '' COMMENT 'token',
  `oid` varchar(32) NOT NULL DEFAULT '' COMMENT '关联组织id',
  PRIMARY KEY (`id`),
  KEY `uid` (`uid`),
  KEY `title` (`title`),
  KEY `oid` (`oid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_project_relevance` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `uid` bigint(20) NOT NULL COMMENT '关联用户id',
  `project_id` bigint(20) NOT NULL COMMENT '关联项目id',
  `create_time` bigint(20) NOT NULL COMMENT '创建时间',
  `role` varchar(32) DEFAULT NULL COMMENT '用户基于项目的角色',
  PRIMARY KEY (`id`),
  KEY `uid` (`uid`),
  KEY `project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_task_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL COMMENT '关联项目id',
  `task_id` varchar(32) NOT NULL COMMENT '关联任务id',
  `project` varchar(100) NOT NULL COMMENT '项目名称',
  `name` varchar(100) NOT NULL COMMENT '任务名称',
  `result` text NOT NULL COMMENT '任务执行结果',
  `start_time` bigint(20) NOT NULL COMMENT '任务开始时间',
  `end_time` bigint(20) NOT NULL COMMENT '任务结束时间',
  `command` varchar(1000) NOT NULL COMMENT '任务指令',
  `with_error` int(11) NOT NULL COMMENT '是否发生错误',
  `client_ip` varchar(20) NOT NULL COMMENT '节点ip',
  `agent_version` varchar(50) NOT NULL DEFAULT '' COMMENT '执行该任务agent的版本',
  `tmp_id` varchar(50) NOT NULL COMMENT '任务执行id',
  `plan_time` bigint(20) NOT NULL DEFAULT '0' COMMENT '任务的计划执行时间',
  PRIMARY KEY (`id`),
  KEY `task_id` (`task_id`),
  KEY `name` (`name`),
  KEY `client_ip` (`client_ip`),
  KEY `start_time` (`start_time`),
  KEY `end_time` (`end_time`),
  KEY `plan_time` (`plan_time`) USING BTREE,
  KEY `pid_tid_tmpid` (`project_id`,`task_id`,`tmp_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_user` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL COMMENT '用户名称',
  `permission` varchar(100) NOT NULL COMMENT '用户权限',
  `account` varchar(100) NOT NULL COMMENT '用户账号',
  `password` varchar(255) NOT NULL COMMENT '用户密码',
  `salt` varchar(6) NOT NULL COMMENT '密码盐',
  `create_time` bigint(20) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `name` (`name`),
  KEY `account` (`account`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_user_workflow` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` int(11) NOT NULL COMMENT '关联用户id',
  `workflow_id` int(11) NOT NULL COMMENT '关联workflow id',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `workflow_id` (`workflow_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_webhook` (
  `callback_url` varchar(255) NOT NULL COMMENT '回调地址',
  `project_id` int(11) NOT NULL COMMENT '关联项目id',
  `type` varchar(30) NOT NULL COMMENT 'webhook类型',
  `secret` varchar(32) NOT NULL COMMENT 'secret',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  KEY `type` (`type`),
  KEY `project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_workflow` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `title` varchar(100) NOT NULL COMMENT 'flow标题',
  `remark` text NOT NULL COMMENT 'flow详细介绍',
  `cron` varchar(20) NOT NULL COMMENT 'cron表达式',
  `status` tinyint(1) NOT NULL DEFAULT '2' COMMENT 'workflow状态，1启用2暂停',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  `oid` varchar(32) NOT NULL COMMENT '关联组织id',
  PRIMARY KEY (`id`),
  KEY `oid` (`oid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_workflow_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `workflow_id` int(11) NOT NULL COMMENT '关联workflow id',
  `start_time` int(11) NOT NULL COMMENT '开始时间',
  `end_time` int(11) NOT NULL COMMENT '结束时间',
  `result` text NOT NULL COMMENT '任务执行结果',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `workflow_id` (`workflow_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_workflow_schedule_plan` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `workflow_id` int(11) NOT NULL COMMENT '关联workflow id',
  `task_id` varchar(50) NOT NULL COMMENT 'task id',
  `project_id` int(11) NOT NULL COMMENT 'project id',
  `dependency_task_id` varchar(50) NOT NULL DEFAULT '',
  `dependency_project_id` int(11) NOT NULL DEFAULT '0' COMMENT '依赖任务的项目id',
  `project_task_index` varchar(100) NOT NULL DEFAULT '' COMMENT '项目+任务索引',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `dependency_project_id` (`dependency_project_id`),
  KEY `project_task_index` (`project_task_index`),
  KEY `workflow_id` (`workflow_id`),
  KEY `task_id` (`task_id`),
  KEY `project_id` (`project_id`),
  KEY `dependency_task_id` (`dependency_task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE `gc_workflow_task` (
  `task_id` varchar(50) NOT NULL COMMENT 'task id',
  `project_id` int(11) NOT NULL COMMENT 'project id',
  `task_name` varchar(100) NOT NULL COMMENT '任务名称',
  `command` text COMMENT '执行命令',
  `remark` text COMMENT '任务备注',
  `timeout` int(11) NOT NULL DEFAULT '0' COMMENT '超时时间(s)',
  `noseize` int(11) NOT NULL DEFAULT '0' COMMENT '不抢占，设为1后多个agent并行执行',
  `workflow_id` int(11) NOT NULL COMMENT '关联workflow id',
  `create_time` int(11) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`task_id`),
  KEY `project_id` (`project_id`),
  KEY `task_name` (`task_name`),
  KEY `workflow_id` (`workflow_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE IF NOT EXISTS gc_task_log_p (
  id BIGINT AUTO_INCREMENT,
  project_id BIGINT NOT NULL,
  project VARCHAR(128) NOT NULL,
  task_id VARCHAR(64) NOT NULL,
  tmp_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  command TEXT NOT NULL,
  plan_time BIGINT NOT NULL,
  start_time BIGINT NOT NULL,
  end_time BIGINT NOT NULL DEFAULT 0,
  result TEXT,
  with_error TINYINT(1) NOT NULL DEFAULT 0,
  agent_version VARCHAR(128) NOT NULL DEFAULT '',
  client_ip VARCHAR(64) NOT NULL DEFAULT '',
  create_time BIGINT NOT NULL,
  PRIMARY KEY (id, plan_time),
  UNIQUE KEY uk_project_task_tmp (plan_time, project_id, task_id, tmp_id)
) PARTITION BY RANGE (plan_time) (
  PARTITION p_default VALUES LESS THAN (1)
);