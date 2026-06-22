#!/bin/bash
# ABAC 策略引擎 API 调用示例集

set -e

API_BASE="http://localhost:8080/api/v1"
API_KEY="${API_KEY:-sk-demo-tenant-key-12345}"
ADMIN_TOKEN="${ADMIN_TOKEN:-admin-secret-token-change-me}"

echo ""
echo "=============================================="
echo "  ABAC API 调用示例集"
echo "=============================================="
echo ""

case "${1:-health}" in
    health)
        echo "[1] 健康检查"
        curl -s "$API_BASE/health" | python3 -m json.tool 2>/dev/null || curl -s "$API_BASE/health"
        echo ""
        ;;

    attributes)
        echo "[2] 获取支持的属性和操作符列表"
        curl -s "$API_BASE/attributes" -H "X-API-Key: $API_KEY" | python3 -m json.tool
        echo ""
        ;;

    decide)
        echo "[3] 权限决策请求"
        curl -s -X POST "$API_BASE/decide" \
          -H "Content-Type: application/json" \
          -H "X-API-Key: $API_KEY" \
          -d '{
            "project_id": "proj-finance",
            "subject": {
              "user_id": "u1001",
              "username": "zhang.san",
              "roles": ["finance_manager"],
              "department": "finance",
              "level": 6,
              "tags": ["finance", "proj-finance"],
              "is_admin": false
            },
            "resource": {
              "id": "doc-2024-001",
              "type": "document",
              "name": "2024财务预算.pdf",
              "owner_id": "u9999",
              "owner_dept": "finance",
              "sensitivity_level": "confidential",
              "project_id": "proj-finance",
              "tags": ["finance", "budget"],
              "status": "active"
            },
            "action": "read",
            "environment": {
              "timestamp": "'$(date +%Y-%m-%dT%H:%M:%S%z)'",
              "client_ip": "10.0.1.50",
              "device_type": "desktop",
              "is_mfa_authenticated": true,
              "country": "CN"
            }
          }' | python3 -m json.tool
        echo ""
        ;;

    decide-trace)
        echo "[4] 带决策过程 Trace 的权限决策"
        curl -s -X POST "$API_BASE/decide/trace" \
          -H "Content-Type: application/json" \
          -H "X-API-Key: $API_KEY" \
          -d '{
            "project_id": "proj-finance",
            "subject": {
              "user_id": "u1001",
              "roles": ["finance_manager"],
              "department": "finance",
              "level": 6
            },
            "resource": {
              "type": "document",
              "sensitivity_level": "confidential",
              "tags": ["finance"]
            },
            "action": "export",
            "environment": {
              "timestamp": "'$(date +%Y-%m-%dT%H:%M:%S%z)'",
              "client_ip": "10.0.1.50",
              "is_mfa_authenticated": true
            }
          }' | python3 -m json.tool
        echo ""
        ;;

    simulate)
        echo "[5] What-if 策略模拟（不记录审计日志）"
        curl -s -X POST "$API_BASE/simulate" \
          -H "Content-Type: application/json" \
          -H "X-API-Key: $API_KEY" \
          -d '{
            "subject": {
              "user_id": "u_outside",
              "email_domain": "external.com",
              "department": "guest"
            },
            "resource": {
              "type": "document",
              "id": "doc-001"
            },
            "action": "delete",
            "environment": {
              "timestamp": "'$(date +%Y-%m-%dT%H:%M:%S%z)'",
              "client_ip": "1.2.3.4"
            }
          }' | python3 -m json.tool
        echo ""
        ;;

    policies)
        echo "[6] 获取策略列表"
        curl -s "$API_BASE/policies" -H "X-API-Key: $API_KEY" | python3 -m json.tool
        echo ""
        ;;

    audit)
        echo "[7] 查询审计日志（最近7天）"
        START=$(date -v-7d +%Y-%m-%dT00:00:00 2>/dev/null || date -d "-7 days" +%Y-%m-%dT00:00:00)
        END=$(date +%Y-%m-%dT23:59:59)
        curl -s "$API_BASE/audit?start=${START}&end=${END}&limit=10" \
          -H "X-API-Key: $API_KEY" | python3 -m json.tool
        echo ""
        ;;

    tenants)
        echo "[8] 平台管理员 - 获取租户列表"
        curl -s "$API_BASE/admin/tenants?offset=0&limit=10" \
          -H "X-Admin-Token: $ADMIN_TOKEN" | python3 -m json.tool
        echo ""
        ;;

    create-tenant)
        echo "[9] 平台管理员 - 创建新租户"
        TENANT_ID="tenant-$(date +%s | tail -c 6)"
        curl -s -X POST "$API_BASE/admin/tenants" \
          -H "Content-Type: application/json" \
          -H "X-Admin-Token: $ADMIN_TOKEN" \
          -d "{
            \"id\": \"$TENANT_ID\",
            \"name\": \"示例租户 $TENANT_ID\",
            \"combining_algorithm\": \"deny-override\",
            \"max_policies\": 200,
            \"max_rps\": 500
          }" | python3 -m json.tool
        echo ""
        echo "💡 请保存上方返回的 API Key！"
        echo ""
        ;;

    validate)
        echo "[10] 校验策略 YAML 合法性"
        curl -s -X POST "$API_BASE/policies/validate" \
          -H "Content-Type: application/json" \
          -H "X-API-Key: $API_KEY" \
          -d '{
            "yaml": "id: test-pol-1\ndescription: 测试策略\nlevel: tenant\ntarget:\n  action:\n    conditions:\n      - attribute: name\n        operator: equals\n        value: read\neffect: permit\npriority: 100\nstatus: enabled"
          }' | python3 -m json.tool
        echo ""
        ;;

    stress)
        echo "[11] 简单压测 - 100次并发决策请求"
        echo "开始..."
        START_T=$(date +%s%N)
        for i in $(seq 1 100); do
          curl -s -o /dev/null -X POST "$API_BASE/decide" \
            -H "Content-Type: application/json" \
            -H "X-API-Key: $API_KEY" \
            -d '{"subject":{"roles":["admin"],"is_admin":true},"resource":{"type":"document"},"action":"read","environment":{"client_ip":"10.0.0.1"}}' &
        done
        wait
        END_T=$(date +%s%N)
        ELAPSED=$(( (END_T - START_T) / 1000000 ))
        echo "✅ 完成 100 次请求，耗时 ${ELAPSED}ms，平均每次 ${ELAPSED}ms/100 = $(($ELAPSED/100))ms"
        echo ""
        ;;

    *)
        echo "用法: $0 {health|attributes|decide|decide-trace|simulate|policies|audit|tenants|create-tenant|validate|stress}"
        echo ""
        echo "示例："
        echo "  $0 health              # 健康检查"
        echo "  $0 decide              # 决策请求"
        echo "  $0 decide-trace        # 带Trace的决策"
        echo "  $0 simulate            # 策略模拟"
        echo "  $0 policies            # 策略列表"
        echo "  $0 audit               # 审计日志"
        echo "  $0 tenants             # 租户列表"
        echo "  $0 create-tenant       # 创建租户"
        echo "  $0 validate            # 校验策略YAML"
        echo "  $0 stress              # 简单压测"
        echo ""
        echo "可通过环境变量覆盖凭证："
        echo "  API_KEY=xxx $0 decide"
        echo "  ADMIN_TOKEN=xxx $0 tenants"
        exit 1
        ;;
esac
