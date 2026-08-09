-- Migration: 207_channel_monitor_models_probe
-- 渠道监控统一改为 GET /v1/models 目录探测，避免周期性真实推理消耗上游 token。

ALTER TABLE channel_monitors
    DROP CONSTRAINT IF EXISTS channel_monitors_api_mode_check;

ALTER TABLE channel_monitor_request_templates
    DROP CONSTRAINT IF EXISTS channel_monitor_request_templates_api_mode_check;

UPDATE channel_monitors
SET api_mode = 'models'
WHERE api_mode IS DISTINCT FROM 'models';

UPDATE channel_monitor_request_templates
SET api_mode = 'models'
WHERE api_mode IS DISTINCT FROM 'models';

ALTER TABLE channel_monitors
    ALTER COLUMN api_mode SET DEFAULT 'models';

ALTER TABLE channel_monitor_request_templates
    ALTER COLUMN api_mode SET DEFAULT 'models';

ALTER TABLE channel_monitors
    ADD CONSTRAINT channel_monitors_api_mode_check CHECK (api_mode = 'models');

ALTER TABLE channel_monitor_request_templates
    ADD CONSTRAINT channel_monitor_request_templates_api_mode_check CHECK (api_mode = 'models');
