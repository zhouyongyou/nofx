#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - Docker Quick Start Script
# Usage: ./start.sh [command]
# ═══════════════════════════════════════════════════════════════

set -e

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ------------------------------------------------------------------------
# Utility Functions: Colored Output
# ------------------------------------------------------------------------
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

generate_jwt_secret() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 48 | tr -d '\n'
    else
        head -c 128 /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 64
    fi
}

ensure_jwt_secret_in_config() {
    if [ ! -f "config.json" ]; then
        return
    fi
    local current="" default="" secret tmp
    if command -v jq >/dev/null 2>&1; then
        current=$(jq -r '.jwt_secret // ""' config.json 2>/dev/null || echo "")
        if [ -f "config.json.example" ]; then
            default=$(jq -r '.jwt_secret // ""' config.json.example 2>/dev/null || echo "")
        fi
        if [ -z "$current" ] || { [ -n "$default" ] && [ "$current" = "$default" ]; }; then
            secret=$(generate_jwt_secret)
            tmp=$(mktemp)
            jq --arg s "$secret" '.jwt_secret = $s' config.json > "$tmp" && mv "$tmp" config.json
            print_success "已生成并写入 jwt_secret"
        fi
    else
        secret=$(generate_jwt_secret)
        tmp=$(mktemp)
        sed -E "s/(\"jwt_secret\"[[:space:]]*:[[:space:]]*\")[^"]*(\")/\1${secret}\2/" config.json > "$tmp" && mv "$tmp" config.json
        print_success "已生成并写入 jwt_secret"
    fi
}

sync_env_jwt_secret() {
    if [ ! -f "config.json" ]; then
        return
    fi
    local secret=""
    if command -v jq >/dev/null 2>&1; then
        secret=$(jq -r '.jwt_secret // ""' config.json 2>/dev/null || echo "")
    else
        secret=$(grep -o '"jwt_secret"[[:space:]]*:[[:space:]]*"[^"]*"' config.json | sed -E 's/.*:\s*\"(.*)\"/\1/' )
    fi
    if [ -z "$secret" ]; then
        return
    fi
    if [ -f ".env" ]; then
        if grep -q '^JWT_SECRET=' .env; then
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=${secret}|" .env
            else
                sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${secret}|" .env
            fi
        else
            echo "JWT_SECRET=${secret}" >> .env
        fi
        chmod 600 .env 2>/dev/null || true
        print_success "已同步 .env 中的 JWT_SECRET"
    fi
}

detect_hardware() {
    CPU_CORES=1
    TOTAL_MEM_GB=1
    if command -v nproc >/dev/null 2>&1; then
        CPU_CORES=$(nproc)
    elif command -v sysctl >/dev/null 2>&1; then
        CPU_CORES=$(sysctl -n hw.ncpu 2>/dev/null || echo 1)
    fi
    if [ -r /proc/meminfo ]; then
        TOTAL_MEM_GB=$(awk '/MemTotal/ {printf "%d", $2/1024/1024}' /proc/meminfo)
    elif command -v sysctl >/dev/null 2>&1; then
        TOTAL_MEM_GB=$(sysctl -n hw.memsize 2>/dev/null | awk '{printf "%d", $1/1024/1024/1024}')
    fi
    [ -z "$CPU_CORES" ] && CPU_CORES=1
    [ -z "$TOTAL_MEM_GB" ] && TOTAL_MEM_GB=1
}

apply_resource_limits() {
    detect_hardware
    local backend_cpus frontend_cpus backend_mem frontend_mem half
    if [ "$CPU_CORES" -ge 8 ]; then
        backend_cpus=4
    elif [ "$CPU_CORES" -ge 4 ]; then
        backend_cpus=3
    elif [ "$CPU_CORES" -ge 2 ]; then
        backend_cpus=1.5
    else
        backend_cpus=1
    fi
    frontend_cpus=0.5
    half=$((TOTAL_MEM_GB/2))
    if [ "$half" -lt 1 ]; then
        backend_mem=1g
    elif [ "$half" -gt 8 ]; then
        backend_mem=8g
    else
        backend_mem="${half}g"
    fi
    frontend_mem=256m
    if docker ps --format '{{.Names}}' | grep -q '^nofx-trading$'; then
        docker update --cpus "$backend_cpus" --memory "$backend_mem" nofx-trading >/dev/null 2>&1 || true
    fi
    if docker ps --format '{{.Names}}' | grep -q '^nofx-frontend$'; then
        docker update --cpus "$frontend_cpus" --memory "$frontend_mem" nofx-frontend >/dev/null 2>&1 || true
    fi
    print_info "已根据硬件参数应用资源限额"
}
# ------------------------------------------------------------------------
# Detection: Docker Compose Command (Backward Compatible)
# ------------------------------------------------------------------------
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose 未安装！请先安装 Docker Compose"
        exit 1
    fi
    print_info "使用 Docker Compose 命令: $COMPOSE_CMD"
}

# ------------------------------------------------------------------------
# Validation: Docker Installation
# ------------------------------------------------------------------------
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装！请先安装 Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    detect_compose_cmd
    print_success "Docker 和 Docker Compose 已安装"
}

# ------------------------------------------------------------------------
# Validation: Environment File (.env)
# ------------------------------------------------------------------------
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env 不存在，从模板复制..."
        cp .env.example .env
        print_info "✓ 已使用默认环境变量创建 .env"
        print_info "💡 如需修改端口等设置，可编辑 .env 文件"
    fi
    print_success "环境变量文件存在"
}

# ------------------------------------------------------------------------
# Validation: Encryption Environment (RSA Keys + Data Encryption Key)
# ------------------------------------------------------------------------
check_encryption() {
    local need_setup=false
    
    print_info "检查加密环境..."
    
    # 检查RSA密钥对
    if [ ! -f "secrets/rsa_key" ] || [ ! -f "secrets/rsa_key.pub" ]; then
        print_warning "RSA密钥对不存在"
        need_setup=true
    fi
    
    # 检查数据加密密钥
    if [ ! -f ".env" ] || ! grep -q "^DATA_ENCRYPTION_KEY=" .env; then
        print_warning "数据加密密钥未配置"
        need_setup=true
    fi
    
    # 检查JWT认证密钥
    if [ ! -f ".env" ] || ! grep -q "^JWT_SECRET=" .env; then
        print_warning "JWT认证密钥未配置"
        need_setup=true
    fi
    
    # 如果需要设置加密环境，直接自动设置
    if [ "$need_setup" = "true" ]; then
        print_info "🔐 检测到加密环境未配置，正在自动设置..."
        print_info "加密环境用于保护敏感数据（API密钥、私钥等）"
        echo ""

        # 检查加密设置脚本是否存在
        if [ -f "scripts/setup_encryption.sh" ]; then
            print_info "加密系统将保护: API密钥、私钥、Hyperliquid代理钱包"
            echo ""

            # 自动运行加密设置脚本
            echo -e "Y\nn\nn" | bash scripts/setup_encryption.sh
            if [ $? -eq 0 ]; then
                echo ""
                print_success "🔐 加密环境设置完成！"
                print_info "  • RSA-2048密钥对已生成"
                print_info "  • AES-256数据加密密钥已配置"
                print_info "  • JWT认证密钥已配置"
                print_info "  • 所有敏感数据现在都受加密保护"
                echo ""
            else
                print_error "加密环境设置失败"
                exit 1
            fi
        else
            print_error "加密设置脚本不存在: scripts/setup_encryption.sh"
            print_info "请手动运行: ./scripts/setup_encryption.sh"
            exit 1
        fi
    else
        print_success "🔐 加密环境已配置"
        print_info "  • RSA密钥对: secrets/rsa_key + secrets/rsa_key.pub"
        print_info "  • 数据加密密钥: .env (DATA_ENCRYPTION_KEY)"
        print_info "  • JWT认证密钥: .env (JWT_SECRET)"
        print_info "  • 加密算法: RSA-OAEP-2048 + AES-256-GCM + HS256"
        print_info "  • 保护数据: API密钥、私钥、Hyperliquid代理钱包、用户认证"
        
        # 验证密钥文件权限
        if [ -f "secrets/rsa_key" ]; then
            local perm=$(stat -f "%A" "secrets/rsa_key" 2>/dev/null || stat -c "%a" "secrets/rsa_key" 2>/dev/null)
            if [ "$perm" != "600" ]; then
                print_warning "修复RSA私钥权限..."
                chmod 600 secrets/rsa_key
            fi
        fi
        
        if [ -f ".env" ]; then
            local perm=$(stat -f "%A" ".env" 2>/dev/null || stat -c "%a" ".env" 2>/dev/null)
            if [ "$perm" != "600" ]; then
                print_warning "修复环境文件权限..."
                chmod 600 .env
            fi
        fi
    fi
}

# ------------------------------------------------------------------------
# Validation: Configuration File (config.json) - BASIC SETTINGS ONLY
# ------------------------------------------------------------------------
check_config() {
    if [ ! -f "config.json" ]; then
        print_warning "config.json 不存在，从模板复制..."
        cp config.json.example config.json
        print_info "✓ 已使用默认配置创建 config.json"
        print_info "💡 如需修改基础设置（杠杆大小、开仓币种、管理员模式、JWT密钥等），可编辑 config.json"
        print_info "💡 模型/交易所/交易员配置请使用Web界面"
    fi
    print_success "配置文件存在"
}

# ------------------------------------------------------------------------
# Validation: Database File (config.db)
# ------------------------------------------------------------------------
check_database() {
    if [ -f "scripts/init-db.sh" ]; then
        ./scripts/init-db.sh
    else
        # 簡單備用檢查
        if [ -d "config.db" ]; then
            print_warning "config.db 是目錄，正在修復..."
            mv config.db "config.db.broken_$(date +%Y%m%d_%H%M%S)"
            touch config.db
            print_success "已修復 config.db"
        elif [ ! -e "config.db" ]; then
            touch config.db
            print_info "已創建空的 config.db"
        fi
    fi
}

# ------------------------------------------------------------------------
# Utility: Read Environment Variables
# ------------------------------------------------------------------------
read_env_vars() {
    if [ -f ".env" ]; then
        # 读取端口配置，设置默认值
        NOFX_FRONTEND_PORT=$(grep "^NOFX_FRONTEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "3000")
        NOFX_BACKEND_PORT=$(grep "^NOFX_BACKEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "8080")
        
        # 去除可能的引号和空格
        NOFX_FRONTEND_PORT=$(echo "$NOFX_FRONTEND_PORT" | tr -d '"'"'" | tr -d ' ')
        NOFX_BACKEND_PORT=$(echo "$NOFX_BACKEND_PORT" | tr -d '"'"'" | tr -d ' ')
        
        # 如果为空则使用默认值
        NOFX_FRONTEND_PORT=${NOFX_FRONTEND_PORT:-3000}
        NOFX_BACKEND_PORT=${NOFX_BACKEND_PORT:-8080}
    else
        # 如果.env不存在，使用默认端口
        NOFX_FRONTEND_PORT=3000
        NOFX_BACKEND_PORT=8080
    fi
}

# ------------------------------------------------------------------------
# Validation: Database File (config.db)
# ------------------------------------------------------------------------
check_database() {
    if [ -d "config.db" ]; then
        # 如果存在的是目录，删除它
        print_warning "config.db 是目录而非文件，正在删除目录..."
        rm -rf config.db
        print_info "✓ 已删除目录，现在创建文件..."
        install -m 600 /dev/null config.db
        print_success "✓ 已创建空数据库文件（权限: 600），系统将在启动时初始化"
    elif [ ! -f "config.db" ]; then
        # 如果不存在文件，创建它
        print_warning "数据库文件不存在，创建空数据库文件..."
        # 创建空文件以避免Docker创建目录（使用安全权限600）
        install -m 600 /dev/null config.db
        print_info "✓ 已创建空数据库文件（权限: 600），系统将在启动时初始化"
    else
        # 文件存在
        print_success "数据库文件存在"

        # 检查是否需要数据库迁移（z-dev-v2 多配置架构升级）
        if command -v sqlite3 &> /dev/null && [ -s "config.db" ]; then
            print_info "检查数据库 schema 版本..."

            # 检查是否存在旧的列（ai_model_id_old, exchange_id_old）
            local has_old_columns=$(sqlite3 config.db "PRAGMA table_info(traders);" 2>/dev/null | grep -c "_old" || echo "0")

            if [ "$has_old_columns" -gt 0 ]; then
                print_warning "⚠️  检测到数据库 schema 需要迁移！"
                print_warning "   发现 $has_old_columns 个旧列（ai_model_id_old, exchange_id_old）"
                print_warning "   这会导致创建交易员失败（500 错误）"
                echo ""
                print_info "🔧 自动修复选项："
                print_info "   运行: ./scripts/fix_traders_table_migration.sh config.db"
                echo ""
                print_warning "❌ 如果不修复，创建交易员将失败！"
                echo ""

                # 询问是否自动修复
                if [ -f "scripts/fix_traders_table_migration.sh" ]; then
                    read -p "$(echo -e ${YELLOW})是否自动修复数据库? (y/n): $(echo -e ${NC})" -n 1 -r
                    echo
                    if [[ $REPLY =~ ^[Yy]$ ]]; then
                        print_info "正在运行数据库修复脚本..."
                        if bash scripts/fix_traders_table_migration.sh config.db; then
                            print_success "✅ 数据库修复成功！"
                        else
                            print_error "❌ 数据库修复失败，请查看错误信息"
                            exit 1
                        fi
                    else
                        print_warning "跳过自动修复，请手动运行修复脚本"
                        print_info "继续启动可能会导致创建交易员失败"
                    fi
                else
                    print_error "修复脚本不存在: scripts/fix_traders_table_migration.sh"
                    print_info "请从最新版本拉取此文件"
                fi
            else
                print_success "✅ 数据库 schema 版本正确"
            fi
        fi
    fi
}

# ------------------------------------------------------------------------
# Build: Frontend (Node.js Based)
# ------------------------------------------------------------------------
# build_frontend() {
#     print_info "检查前端构建环境..."

#     if ! command -v node &> /dev/null; then
#         print_error "Node.js 未安装！请先安装 Node.js"
#         exit 1
#     fi

#     if ! command -v npm &> /dev/null; then
#         print_error "npm 未安装！请先安装 npm"
#         exit 1
#     fi

#     print_info "正在构建前端..."
#     cd web

#     print_info "安装 Node.js 依赖..."
#     npm install

#     print_info "构建前端应用..."
#     npm run build

#     cd ..
#     print_success "前端构建完成"
# }

# ------------------------------------------------------------------------
# Service Management: Start
# ------------------------------------------------------------------------
start() {
    print_info "正在启动 NOFX AI Trading System..."

    # 读取环境变量
    read_env_vars

    # 确保必要的文件和目录存在（修复 Docker volume 挂载问题）
    if [ ! -f "config.db" ]; then
        print_info "创建数据库文件..."
        install -m 600 /dev/null config.db
    fi
    if [ ! -d "decision_logs" ]; then
        print_info "创建日志目录..."
        install -m 700 -d decision_logs
    fi

    # Auto-build frontend if missing or forced
    # if [ ! -d "web/dist" ] || [ "$1" == "--build" ]; then
    #     build_frontend
    # fi

    # Rebuild images if flag set
    if [ "$1" == "--build" ]; then
        print_info "重新构建镜像..."
        $COMPOSE_CMD up -d --build
    else
        print_info "启动容器..."
        $COMPOSE_CMD up -d
    fi

    apply_resource_limits

    print_success "服务已启动！"
    print_info "Web 界面: http://localhost:${NOFX_FRONTEND_PORT}"
    print_info "API 端点: http://localhost:${NOFX_BACKEND_PORT}"
    print_info ""
    print_info "查看日志: ./start.sh logs"
    print_info "停止服务: ./start.sh stop"
}

# ------------------------------------------------------------------------
# Service Management: Stop
# ------------------------------------------------------------------------
stop() {
    print_info "正在停止服务..."
    $COMPOSE_CMD stop
    print_success "服务已停止"
}

# ------------------------------------------------------------------------
# Service Management: Restart
# ------------------------------------------------------------------------
restart() {
    print_info "正在重启服务..."
    $COMPOSE_CMD restart
    print_success "服务已重启"
}

# ------------------------------------------------------------------------
# Monitoring: Logs
# ------------------------------------------------------------------------
logs() {
    if [ -z "$2" ]; then
        $COMPOSE_CMD logs -f
    else
        $COMPOSE_CMD logs -f "$2"
    fi
}

# ------------------------------------------------------------------------
# Monitoring: Status
# ------------------------------------------------------------------------
status() {
    # 读取环境变量
    read_env_vars
    
    print_info "服务状态:"
    $COMPOSE_CMD ps
    echo ""
    print_info "健康检查:"
    curl -s "http://localhost:${NOFX_BACKEND_PORT}/api/health" | jq '.' || echo "后端未响应"
}

# ------------------------------------------------------------------------
# Maintenance: Clean (Destructive)
# ------------------------------------------------------------------------
clean() {
    print_warning "这将删除所有容器和数据！"
    read -p "确认删除？(yes/no): " confirm
    if [ "$confirm" == "yes" ]; then
        print_info "正在清理..."
        $COMPOSE_CMD down -v
        print_success "清理完成"
    else
        print_info "已取消"
    fi
}

# ------------------------------------------------------------------------
# Maintenance: Update
# ------------------------------------------------------------------------
update() {
    print_info "正在更新..."
    git pull
    $COMPOSE_CMD up -d --build
    print_success "更新完成"
}

# ------------------------------------------------------------------------
# Encryption: Manual Setup
# ------------------------------------------------------------------------
setup_encryption_manual() {
    print_info "🔐 手动设置加密环境"
    
    if [ -f "scripts/setup_encryption.sh" ]; then
        bash scripts/setup_encryption.sh
    else
        print_error "加密设置脚本不存在: scripts/setup_encryption.sh"
        print_info "请确保项目文件完整"
        exit 1
    fi
}

# ------------------------------------------------------------------------
# Help: Usage Information
# ------------------------------------------------------------------------
show_help() {
    echo "NOFX AI Trading System - Docker 管理脚本"
    echo ""
    echo "用法: ./start.sh [command] [options]"
    echo ""
    echo "命令:"
    echo "  start [--build]    启动服务（可选：重新构建）"
    echo "  stop               停止服务"
    echo "  restart            重启服务"
    echo "  logs [service]     查看日志（可选：指定服务名 backend/frontend）"
    echo "  status             查看服务状态"
    echo "  clean              清理所有容器和数据"
    echo "  update             更新代码并重启"
    echo "  setup-encryption   设置加密环境（RSA密钥+数据加密）"
    echo "  help               显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  ./start.sh start --build    # 构建并启动"
    echo "  ./start.sh logs backend     # 查看后端日志"
    echo "  ./start.sh status           # 查看状态"
    echo "  ./start.sh setup-encryption # 手动设置加密环境"
    echo ""
    echo "🔐 关于加密:"
    echo "  系统自动检测加密环境，首次运行时会自动设置"
    echo "  手动设置: ./scripts/setup_encryption.sh"
}

# ------------------------------------------------------------------------
# Main: Command Dispatcher
# ------------------------------------------------------------------------
main() {
    check_docker

    case "${1:-start}" in
        start)
            check_env
            check_encryption
            check_config
            ensure_jwt_secret_in_config
            sync_env_jwt_secret
            check_database
            start "$2"
            ;;
        stop)
            stop
            ;;
        restart)
            restart
            ;;
        logs)
            logs "$@"
            ;;
        status)
            status
            ;;
        clean)
            clean
            ;;
        update)
            update
            ;;
        setup-encryption)
            setup_encryption_manual
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# Execute Main
main "$@"