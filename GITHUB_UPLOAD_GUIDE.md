# GitHub 上传指南

## 方法一：使用提供的脚本（推荐）

在项目根目录执行：

```bash
chmod +x upload_to_github.sh
./upload_to_github.sh
```

脚本会自动完成以下步骤：
1. 初始化 Git 仓库
2. 添加所有文件
3. 创建提交
4. 设置主分支为 main
5. 添加远程仓库
6. 推送到 GitHub

---

## 方法二：手动执行命令

### 步骤 1: 初始化 Git 仓库

```bash
cd /Users/bruce/gowork/dt-server
git init
```

### 步骤 2: 添加所有文件

```bash
git add .
```

### 步骤 3: 创建提交

```bash
git commit -m "Initial commit: Dragon vs Tiger game server

Features:
- Core game APIs: /api/game_event, /api/bet, /api/drawresult
- Multi-platform authentication (B2B2C model)
- Settlement idempotency protection (3-layer)
- Production security fixes
- Log optimization (51% reduction)
- Comprehensive documentation
- MySQL 8.0 + Redis 7 + RocketMQ support"
```

### 步骤 4: 设置主分支

```bash
git branch -M main
```

### 步骤 5: 添加远程仓库

```bash
git remote add origin https://github.com/brucegooo/dt-game.git
```

### 步骤 6: 推送到 GitHub

```bash
git push -u origin main
```

---

## 认证方式

### 方式 1: HTTPS + Personal Access Token（推荐）

1. **创建 Personal Access Token**:
   - 访问: https://github.com/settings/tokens
   - 点击 "Generate new token" → "Generate new token (classic)"
   - 选择权限: `repo` (完整仓库访问权限)
   - 生成并复制 token

2. **使用 Token 推送**:
   ```bash
   git push -u origin main
   ```
   - 用户名: 您的 GitHub 用户名
   - 密码: 粘贴您的 Personal Access Token（不是 GitHub 密码）

### 方式 2: SSH（更安全）

1. **生成 SSH 密钥**（如果还没有）:
   ```bash
   ssh-keygen -t ed25519 -C "your_email@example.com"
   ```

2. **添加 SSH 密钥到 GitHub**:
   ```bash
   # 复制公钥
   cat ~/.ssh/id_ed25519.pub
   ```
   - 访问: https://github.com/settings/keys
   - 点击 "New SSH key"
   - 粘贴公钥内容

3. **更改远程仓库 URL 为 SSH**:
   ```bash
   git remote set-url origin git@github.com:brucegooo/dt-game.git
   ```

4. **推送**:
   ```bash
   git push -u origin main
   ```

---

## 常见问题

### 问题 1: 远程仓库不存在

**错误信息**:
```
remote: Repository not found.
fatal: repository 'https://github.com/brucegooo/dt-game.git/' not found
```

**解决方案**:
1. 访问 https://github.com/new
2. 创建名为 `dt-game` 的新仓库
3. **不要**初始化 README、.gitignore 或 license
4. 创建后再执行推送命令

### 问题 2: 认证失败

**错误信息**:
```
remote: Support for password authentication was removed on August 13, 2021.
fatal: Authentication failed
```

**解决方案**:
使用 Personal Access Token 代替密码（见上面的认证方式）

### 问题 3: 远程仓库已存在内容

**错误信息**:
```
! [rejected]        main -> main (fetch first)
error: failed to push some refs to 'https://github.com/brucegooo/dt-game.git'
```

**解决方案**:
```bash
# 强制推送（会覆盖远程内容）
git push -u origin main --force

# 或者先拉取再推送
git pull origin main --allow-unrelated-histories
git push -u origin main
```

### 问题 4: 文件太大

**错误信息**:
```
remote: error: File xxx is 100.00 MB; this exceeds GitHub's file size limit of 100 MB
```

**解决方案**:
```bash
# 添加到 .gitignore
echo "dt-server" >> .gitignore
echo "*.log" >> .gitignore

# 重新提交
git rm --cached dt-server
git commit --amend
git push -u origin main --force
```

---

## 验证上传成功

上传成功后，访问以下 URL 查看您的项目：

```
https://github.com/brucegooo/dt-game
```

您应该能看到：
- ✅ 所有源代码文件
- ✅ README.md 文件
- ✅ docs/ 目录下的所有文档
- ✅ 配置文件和数据库脚本

---

## 后续操作

### 1. 添加 .gitignore

创建 `.gitignore` 文件，忽略不需要上传的文件：

```bash
cat > .gitignore << 'EOF'
# 编译产物
dt-server
*.exe
*.dll
*.so
*.dylib

# 测试文件
*.test
*.out

# 依赖
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# 日志
*.log
logs/

# 配置文件（包含敏感信息）
config/prod.json
config/local.json

# 临时文件
tmp/
temp/
*.tmp

# 备份文件
*.backup.*
*.bak

# macOS
.DS_Store

# Windows
Thumbs.db
EOF
```

### 2. 更新 README.md

确保 README.md 包含：
- ✅ 项目简介
- ✅ 安装说明
- ✅ 使用方法
- ✅ API 文档链接
- ✅ 贡献指南

### 3. 添加 LICENSE

```bash
# 例如：MIT License
cat > LICENSE << 'EOF'
MIT License

Copyright (c) 2025 Bruce Guo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF
```

### 4. 提交更新

```bash
git add .gitignore LICENSE
git commit -m "Add .gitignore and LICENSE"
git push
```

---

## 快速命令参考

```bash
# 查看状态
git status

# 查看远程仓库
git remote -v

# 查看提交历史
git log --oneline

# 拉取最新代码
git pull

# 推送更新
git push

# 创建新分支
git checkout -b feature/new-feature

# 切换分支
git checkout main

# 合并分支
git merge feature/new-feature
```

---

## 需要帮助？

如果遇到问题，请：
1. 检查 GitHub 仓库是否已创建
2. 确认您有仓库的写入权限
3. 检查网络连接
4. 查看 Git 错误信息

或者联系我获取更多帮助！

---

**最后更新**: 2025-10-20  
**项目地址**: https://github.com/brucegooo/dt-game

