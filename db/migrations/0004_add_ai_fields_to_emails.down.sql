ALTER TABLE emails
  DROP COLUMN IF EXISTS summary,
  DROP COLUMN IF EXISTS priority,
  DROP COLUMN IF EXISTS priority_score,
  DROP COLUMN IF EXISTS ai_sum_model,
  DROP COLUMN IF EXISTS ai_cls_model,
  DROP COLUMN IF EXISTS ai_updated_at;
