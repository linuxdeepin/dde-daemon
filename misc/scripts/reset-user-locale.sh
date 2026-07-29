#!/bin/bash
# 遍历所有普通用户, 删除 ~/.dde_env 和 ~/.config/locale.conf 中的 LANG/LANGUAGE 配置

# 如果文件是普通文件且非软链接，执行 sed 删除匹配行
sed_safe() {
    [ -f "$1" ] && [ ! -L "$1" ] && sed -i "$2" "$1"
}

clean_user_locale() {
    sed_safe "$1/.dde_env" '/^export LANG=/d; /^export LANGUAGE=/d'
    if [ -d "$1/.config" ] && [ ! -L "$1/.config" ]; then
        sed_safe "$1/.config/locale.conf" '/^LANG=/d; /^LANGUAGE=/d'
    fi
}

getent passwd | awk -F: '$3 >= 1000 {print $6}' | while read -r home_dir; do
    [ -z "$home_dir" ] && continue
    [ "$home_dir" = "/" ] && continue
    [ -d "$home_dir" ] && [ ! -L "$home_dir" ] || continue
    clean_user_locale "$home_dir"
done
