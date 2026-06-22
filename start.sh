#!/bin/bash
# ABAC 策略引擎一键启动脚本

set -e

echo "=============================================="
echo "  ABAC 策略引擎 & 权限决策服务"
echo "=============================================="
echo ""

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"

check_docker() {
    if ! command -v docker &> /dev/null; then
        echo "❌ 请先安装 Docker 和 Docker Compose"
        exit 1
    fi
    if ! command -v docker compose &> /dev/null && ! command -v docker-compose &> /dev/null; then
        echo "❌ 请先安装 Docker Compose"
        exit 1
    fi
}

COMPOSE_CMD="docker compose"
if ! command -v docker compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
fi

case "${1:-up}" in
    up)
        check_docker
        echo "🚀 正在启动所有服务..."
        $COMPOSE_CMD up -d --build
        echo ""
        echo "✅ 所有服务已启动！"
        echo ""
        echo "📋 服务列表："
        echo "   - 前端控制台:    http://localhost"
        echo "   - 后端 API:      http://localhost:8080"
        echo "   - PostgreSQL:    localhost:5432"
        echo "   - Redis:         localhost:6379"
        echo ""
        echo "🔑 默认凭证："
        echo "   - Demo 租户 API Key: sk-demo-tenant-key-12345"
        echo "   - 平台管理员 Token:  admin-secret-token-change-me"
        echo "   - PostgreSQL:        abac / abac123"
        echo ""
        echo "💡 查看日志:  ./start.sh logs"
        echo "💡 停止服务:  ./start.sh down"
        ;;

    down)
        check_docker
        echo "🛑 正在停止所有服务..."
        $COMPOSE_CMD down
        echo "✅ 所有服务已停止"
        ;;

    restart)
        check_docker
        echo "🔄 正在重启所有服务..."
        $COMPOSE_CMD restart
        echo "✅ 所有服务已重启"
        ;;

    logs)
        check_docker
        SERVICE="${2:-}"
        if [ -n "$SERVICE" ]; then
            $COMPOSE_CMD logs -f "$SERVICE"
        else
            $COMPOSE_CMD logs -f
        fi
        ;;

    ps)
        check_docker
        $COMPOSE_CMD ps
        ;;

    test)
        echo "🧪 运行 API 健康检查..."
        sleep 2
        curl -s http://localhost:8080/api/v1/health || echo "⚠️  后端服务尚未就绪，请稍后重试"
        echo ""
        echo "🧪 前端健康检查..."
        curl -sI http://localhost | head -n 1 || echo "⚠️  前端服务尚未就绪，请稍后重试"
        ;;

    clean)
        check_docker
        read -p "⚠️  此操作将删除所有数据卷（包括数据库数据），确定吗？(y/N) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            $COMPOSE_CMD down -v
            echo "✅ 已清理所有数据"
        else
            echo "已取消"
        fi
        ;;

    dev-backend)
        echo "🔧 以开发模式启动后端 (Go 本地运行)"
        echo "📋 请确保 PostgreSQL 和 Redis 已启动"
        cd "$PROJECT_DIR/backend"
        if [ ! -f ".env" ]; then
            cp "$PROJECT_DIR/.env.example" .env
            echo "✅ 已创建 .env 文件"
        fi
        go mod download
        go run ./cmd/server
        ;;

    dev-frontend)
        echo "🔧 以开发模式启动前端 (Vite 热更新)"
        cd "$PROJECT_DIR/frontend"
        if [ ! -d "node_modules" ]; then
            npm install
        fi
        npm run dev
        ;;

    *)
        echo "用法: $0 {up|down|restart|logs|ps|test|clean|dev-backend|dev-frontend}"
        echo ""
        echo "命令说明："
        echo "  up            构建并启动所有容器（默认）"
        echo "  down          停止并移除所有容器"
        echo "  restart       重启所有服务"
        echo "  logs [svc]    查看服务日志（可选指定服务名: backend/frontend/postgres/redis）"
        echo "  ps            查看容器状态"
        echo "  test          简单健康检查测试"
        echo "  clean         停止服务并删除所有数据卷"
        echo "  dev-backend   本地开发模式运行后端（需本地Go环境）"
        echo "  dev-frontend  本地开发模式运行前端（需本地Node环境）"
        exit 1
        ;;
esac
