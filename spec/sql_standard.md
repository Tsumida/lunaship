# SQL Standard

## Example

```sql
SET NAMES utf8mb4;
CREATE DATABASE IF NOT EXISTS tex DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE tex;

CREATE TABLE IF NOT EXISTS `spot` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增ID',
  `account_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '用户ID',
  `currency` varchar(8) NOT NULL DEFAULT '' COMMENT '币种',
  `deposit` decimal(65,18) NOT NULL DEFAULT '0.000000000000000000' COMMENT '可用资产',
  `frozen` decimal(65,18) NOT NULL DEFAULT '0.000000000000000000' COMMENT '冻结资产',
  `balance` decimal(65,18) NOT NULL DEFAULT '0.000000000000000000' COMMENT '总资产',
  `version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '版本号, 用于并发控制',
  `create_time` bigint unsigned NOT NULL DEFAULT '0' COMMENT '创建时间',
  `update_time` bigint unsigned NOT NULL DEFAULT '0' COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `udx_account_currency` (`account_id`,`currency`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='账户资产表';
```

## Database
- Version: MySQL 8.0+
- `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`

## Tables
- Must Use `Create ... if not exists`.
- Must have comment for table and column.
- Must Use auto-increment `id` as primary key.

## Fields
- Must: Use `bigint unsigned NOT NULL AUTO_INCREMENT` for ID. 
- Must: Use UTC timestamp (sec, ms, us, ns). Ms is ok for most cases.
- Must: Use version for concurrency control.
- Must: Fields that related to money, like balance, frozen, must be Decimal(a, b). No float or double.
- Allways Set default value except for Text.

## Consistency
To avoid implicit type conversions, literals **MUST** use explicit type hints.. For example: 
```sql
-- bad
UPDATE spot set balance = balance + "100.0" where id = 1;

-- good
UPDATE spot set balance = balance + CAST("100.0" as DECIMAL(65, 18)) where id = 1;
```

# Verification 
## Data check
Add a python script under `tmp/data_check` that: 
1. Generate 1000 mapping, with original url in form of `www.baidu.com?ki=vi` where i in [1, 1000].
2. Record `http://127.0.0.1:8080/{short_url}` mapping and counting. 
3. Keep calling `http://127.0.0.1:8080/{short_url}` until all mappings are verified.
4. When test done, check both MySQL and Redis to make sure all data are correct.

