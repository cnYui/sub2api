-- 按当前 gpt-image-2 token 价格回填历史图片分项；缺 token 的旧记录只做成本归因并标记不完整。

UPDATE usage_logs
SET
    input_cost = GREATEST(input_tokens - image_input_tokens, 0)::numeric * 0.000005,
    image_input_cost = image_input_tokens::numeric * 0.000008,
    output_cost = GREATEST(output_tokens - image_output_tokens, 0)::numeric * 0.000010,
    image_output_cost = image_output_tokens::numeric * 0.000030,
    total_cost = (
        GREATEST(input_tokens - image_input_tokens, 0)::numeric * 0.000005
        + image_input_tokens::numeric * 0.000008
        + GREATEST(output_tokens - image_output_tokens, 0)::numeric * 0.000010
        + image_output_tokens::numeric * 0.000030
        + cache_creation_cost
        + cache_read_cost
    ),
    actual_cost = (
        GREATEST(input_tokens - image_input_tokens, 0)::numeric * 0.000005
        + image_input_tokens::numeric * 0.000008
        + GREATEST(output_tokens - image_output_tokens, 0)::numeric * 0.000010
        + image_output_tokens::numeric * 0.000030
        + cache_creation_cost
        + cache_read_cost
    ) * COALESCE(NULLIF(rate_multiplier, 0), 1),
    billing_mode = COALESCE(NULLIF(billing_mode, ''), 'image')
WHERE requested_model = 'gpt-image-2'
  AND model = 'gpt-image-2'
  AND image_output_tokens > 0
  AND (
      inbound_endpoint IN ('/v1/images/generations', '/v1/images/edits')
      OR upstream_endpoint IN ('/v1/images/generations', '/v1/images/edits')
  );

UPDATE usage_logs
SET
    image_output_cost = actual_cost,
    billing_mode = COALESCE(NULLIF(billing_mode, ''), 'image'),
    billing_incomplete = TRUE
WHERE image_count > 0
  AND image_output_tokens = 0
  AND image_output_cost = 0
  AND actual_cost > 0;
