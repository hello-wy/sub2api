#!/bin/bash
# Sub2API 重新部署脚本（拉取新代码后使用）
# 使用方法: cd deploy && bash redeploy.sh

set -e

cd "$(dirname "$0")"

cd deploy/

echo "🔄 重新构建并启动（使用最新代码）..."
docker compose up -d --build

echo ""
echo "⏳ 等待服务启动..."
sleep 5

echo "📋 服务状态:"
docker compose ps

echo ""
echo "✅ 部署完成！访问: http://localhost:${SERVER_PORT:-8080}"
echo "📝 查看日志: docker compose logs -f sub2api"
