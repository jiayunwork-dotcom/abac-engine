-- ABAC 策略引擎 PostgreSQL 初始化脚本
-- 包含：表结构 + 示例租户 + 示例策略

-- 自动建表已在应用启动时完成，此脚本插入示例数据

-- ============== 示例租户 ==============
INSERT INTO tenants (id, name, api_key, combining_algorithm, max_policies, max_rps, created_at, updated_at)
VALUES
  ('tenant-demo', 'Demo 示例租户', 'sk-demo-tenant-key-12345', 'deny-override', 500, 1000, NOW(), NOW()),
  ('tenant-admin', '平台管理租户', 'sk-admin-tenant-key-99999', 'deny-override', 1000, 5000, NOW(), NOW()),
  ('tenant-acme', 'Acme 科技有限公司', 'sk-acme-corp-key-a7f3d9', 'priority-first', 500, 2000, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============== 全局策略 ==============
-- 策略1: 禁止深夜(22:00-06:00)执行批量导出操作（强制deny，不可被下级覆盖）
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'global-deny-night-export',
  '',
  '',
  'global',
  '全局安全策略：禁止深夜(22:00-06:00)执行导出操作（强制拒绝）',
  '{
    "environment": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "timestamp",
          "operator": "time_range",
          "value": {"start": "22:00", "end": "06:00"}
        }
      ]
    },
    "action": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "name",
          "operator": "equals",
          "value": "export"
        }
      ]
    }
  }'::jsonb,
  'deny',
  9999,
  'enabled',
  1,
  TRUE,
  ARRAY['*'],
  ARRAY['export'],
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- 策略2: 非常用IP段访问需要MFA认证
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'global-mfa-required',
  '',
  '',
  'global',
  '全局安全策略：非常用IP段访问需要MFA二次验证',
  '{
    "environment": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "client_ip",
          "operator": "ip_in_cidr",
          "value": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
        },
        {
          "attribute": "is_mfa_authenticated",
          "operator": "equals",
          "value": false
        }
      ]
    }
  }'::jsonb,
  'deny',
  9998,
  'enabled',
  1,
  TRUE,
  ARRAY['*'],
  NULL,
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- ============== Demo 租户示例策略 ==============
-- 策略1: 财务部门可以读写财务文档
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'pol-finance-docs-rw',
  'tenant-demo',
  'proj-finance',
  'tenant',
  '财务部门对财务类型文档拥有读写权限',
  '{
    "subject": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "department",
          "operator": "equals",
          "value": "finance"
        }
      ]
    },
    "resource": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "type",
          "operator": "equals",
          "value": "document"
        },
        {
          "attribute": "tags",
          "operator": "contains",
          "value": "finance"
        }
      ]
    },
    "action": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "name",
          "operator": "in",
          "value": ["read", "write"]
        }
      ]
    }
  }'::jsonb,
  'permit',
  500,
  'enabled',
  1,
  FALSE,
  ARRAY['document'],
  ARRAY['read', 'write'],
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- 策略2: 管理员拥有所有资源的全部权限
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'pol-admin-full-access',
  'tenant-demo',
  '',
  'tenant',
  '系统管理员拥有所有资源的全部操作权限',
  '{
    "subject": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "roles",
          "operator": "contains",
          "value": "admin"
        }
      ]
    }
  }'::jsonb,
  'permit',
  1000,
  'enabled',
  1,
  FALSE,
  ARRAY['*'],
  ARRAY['*'],
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- 策略3: 机密文档仅部门主管及以上职级可访问
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'pol-confidential-level',
  'tenant-demo',
  '',
  'tenant',
  '机密等级(sensitivity=confidential)文档仅职级≥6的主管可访问',
  '{
    "resource": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "sensitivity_level",
          "operator": "equals",
          "value": "confidential"
        }
      ]
    },
    "subject": {
      "logic": "OR",
      "conditions": [
        {
          "attribute": "level",
          "operator": "gte",
          "value": 6
        },
        {
          "attribute": "is_admin",
          "operator": "equals",
          "value": true
        }
      ]
    }
  }'::jsonb,
  'permit',
  800,
  'enabled',
  1,
  FALSE,
  NULL,
  ARRAY['read'],
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- 策略4: 禁止外部人员删除文档（安全基线）
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'pol-deny-outsider-delete',
  'tenant-demo',
  '',
  'tenant',
  '禁止外部人员(邮箱域非本公司)执行删除操作',
  '{
    "subject": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "email_domain",
          "operator": "not_equals",
          "value": "company.com"
        }
      ]
    },
    "action": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "name",
          "operator": "equals",
          "value": "delete"
        }
      ]
    }
  }'::jsonb,
  'deny',
  900,
  'enabled',
  1,
  FALSE,
  ARRAY['document', 'report'],
  ARRAY['delete'],
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;

-- 策略5: 项目级 - 仅项目成员可访问项目资源
INSERT INTO policies (
  id, tenant_id, project_id, level, description, target,
  effect, priority, status, version, force_deny, resource_types, actions, created_at, updated_at
) VALUES (
  'proj-member-access',
  'tenant-demo',
  'proj-alpha',
  'project',
  'Alpha项目：仅项目成员标签包含proj-alpha的用户可访问',
  '{
    "subject": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "tags",
          "operator": "contains",
          "value": "proj-alpha"
        }
      ]
    },
    "resource": {
      "logic": "AND",
      "conditions": [
        {
          "attribute": "project_id",
          "operator": "equals",
          "value": "proj-alpha"
        }
      ]
    }
  }'::jsonb,
  'permit',
  600,
  'enabled',
  1,
  FALSE,
  NULL,
  NULL,
  NOW(),
  NOW()
) ON CONFLICT (id, tenant_id) DO NOTHING;
