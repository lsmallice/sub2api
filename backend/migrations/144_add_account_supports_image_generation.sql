-- 账号级生图调度能力标记
-- 分组 allow_image_generation 负责用户权限与计费；
-- accounts.supports_image_generation 负责该账号是否可承接生图请求。

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS supports_image_generation BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN accounts.supports_image_generation IS '账号是否支持承接 OpenAI 生图请求';
