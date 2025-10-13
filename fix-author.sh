#!/bin/bash

# 创建一个新的临时分支
git checkout -b temp-fix-author

# 使用git reset回到第一个提交之前
git reset --hard c0cbc53

# 创建一个新的提交，使用正确的作者信息
git add -A
git commit --author="samuel <xiaoyyprocess@163.com>" -m "Initial commit: Complete MyEtcd distributed key-value store implementation"

# 重新应用第二个提交
git cherry-pick 70f87fa

# 强制推送到远程仓库
git push --force origin temp-fix-author:main

# 切换回main分支并删除临时分支
git checkout main
git branch -D temp-fix-author
