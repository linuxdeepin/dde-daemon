#!/bin/bash

# 在这里设置依赖目录，依赖目录不做统计，该目录在不同项目不一致，dde-api是gobuild
#gThirdpartPath="vendor"
gThirdpartPath="gopath"
#gThirdpartPath="gobuild"
# 设置忽略目录，该目录及其子目录不统计
gIgnorePaths=("gobuild" "gopath" "vendor")

gHTMLFile=./cover_report/index.html

gFuncHit=0
gFuncTotal=0
gRowHit=0
gRowTotal=0

function getGOPath() {
    echo "$(pwd):$(pwd)/${gThirdpartPath}:${GOPATH}"
}

# 依赖目录不做统计
function getThirdpartPath() {
     echo "./${gThirdpartPath}/"
}

# 获取需要统计的目录
function getTestPath() {
    currentDirectory=$(pwd)
    cmd=""
    for dir in "${gIgnorePaths[@]}"
    do
        cmd=$cmd" | grep -v ^_"$currentDirectory/$dir".*"
    done
    cmd="go list ./..."$cmd
    ret=$(eval $cmd)
    echo ${ret//"_"$currentDirectory/.}
}

function getGoDirect() {

    arrDirGo=()
    while read -r line
    do
        arrDirGo[${#arrDirGo[*]}]=$line
    done <<< "$(find ./ -name '*.go' | sed 's/\(.*\)\/[^\/]*.go$/\1/' | sort -u)"

    arrDirGOTest=()
    while read -r line
    do
        arrDirGOTest[${#arrDirGOTest[*]}]=$line
    done <<< "$(find ./ -name '*.go' | sed 's/\(.*\)\/[^\/]*_test.go$/\1/' | sort -u)"

    for dirGo in "${arrDirGo[@]}"
    do
        isFind=0
        for dirGoTest in "${arrDirGOTest[@]}"
        do
            if [ "${dirGo}" == "${dirGoTest}" ]; then
                isFind=1
            fi
        done
        strRet=$(echo "$dirGo" | grep "^$(getThirdpartPath).*")
        if [ "$strRet" != "" ]; then
            isFind=1
        fi
        if [ $isFind == 0 ]; then
            echo "$dirGo"
        fi
    done
}

function makeTest() {
    arrSupplementDir=()
    while read -r line
    do
        if [ "$line" != "" ]; then
            arrSupplementDir[${#arrSupplementDir[*]}]=$line
        fi
    done <<< "$(getGoDirect)"

    for supplementDir in "${arrSupplementDir[@]}"
    do
        supplementFile=${supplementDir}"/auto_generate_temp_go_test.go"
        name=$(grep "^package .*" "${supplementDir}"/*.go | head -n 1 | cut -d ":" -f 2)
        echo "supplement:""${supplementDir}"" - ""${name}"
        echo "${name}" >> "$supplementFile"
    done
    #exit
    echo "gotest dir:$(getTestPath)"

    env GOPATH="$(getGOPath)" go test $(getTestPath)  -cover  -coverprofile=coverage.data
    env GOPATH="$(getGOPath)" go tool cover -func=coverage.data -o coverage.txt
    # 下面命令可以生成查看具体覆盖情况的html文件
    #env GOPATH=$(getGOPath) go tool cover -html=coverage.data -o coverage.html

    find ./ -type f -name "auto_generate_temp_go_test.go" -exec rm -rf {} \;

    while read -r line
    do
        arrData=($line)
        if [ ${#arrData[*]} != 3 ]; then
            continue
        fi
        gRowTotal=$((gRowTotal+arrData[1]))

        if [ "${arrData[1]}" != "0" ] && [ "${arrData[2]}" != "0" ]; then
            gRowHit=$((gRowHit+arrData[1]))
        fi
    done  < coverage.data
    rm coverage.data -f

    while read -r line
    do
        if [ ${line:0-5} != "0.0%" ]; then
            ((gFuncHit++))
        fi
        ((gFuncTotal++))
    done  < coverage.txt
    # 去掉最后一行统计
    ((gFuncHit--))
    ((gFuncTotal--))
}

function genHtml() {

    mkdir -pv ./cover_report
    echo "generate html report."
    curdir=$(pwd)
    dirName=${curdir##*/}

    datetime=$(date +"%Y/%m/%d %H:%M:%S")
    echo "<!DOCTYPE html>"  > $gHTMLFile
    echo "<html>" >> $gHTMLFile
    echo "<head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=utf-8\" /></head>"  >> $gHTMLFile
    echo "<body><h2>$dirName code coverage report - gotest[$datetime]</h2>" >> $gHTMLFile
    echo "<hr/>" >> $gHTMLFile

    rowCoverage=0
    if [ $gRowTotal -ne 0 ];then
        rowCoverage=$(awk -v v1=$gRowHit -v v2=$gRowTotal 'BEGIN{printf "%.2f",v1*100/v2}')
    fi
    funcCoverage=0
    if [ $gFuncTotal -ne 0 ];then
        funcCoverage=$(awk -v v1=$gFuncHit -v v2=$gFuncTotal 'BEGIN{printf "%.2f",v1*100/v2}')
    fi

    echo "<p>total:</p>" >> $gHTMLFile
    echo "<table border=\"1\" bordercolor=\"#000000\" width=\"1000\"  style=\"BORDER-COLLAPSE: collapse\" >" >> $gHTMLFile
    echo "<tr style=\"color:White\" bgColor=#0066CC><th></th><th>Hit</th><th>Total</th><th>Coverage</th></tr>" >> $gHTMLFile
    echo "<tr align=\"center\" ><td>Lines</td><td>$gRowHit</td><td>$gRowTotal</td><td>$rowCoverage%</td></tr>" >> $gHTMLFile
    echo "<tr align=\"center\" ><td>Functions</td><td>$gFuncHit</td><td>$gFuncTotal</td><td>$funcCoverage%</td></tr>" >> $gHTMLFile
    echo "</table>" >> $gHTMLFile

    echo "<p>per function:</p>" >> $gHTMLFile
    echo "<table border=\"1\" bordercolor=\"#000000\" width=\"1000\"  style=\"BORDER-COLLAPSE: collapse\" >" >> $gHTMLFile
    echo "<tr style=\"color:White\" bgColor=#0066CC><th>go文件</th><th>函数</th><th>覆盖率</th></tr>" >> $gHTMLFile
    while read line  || [[ -n ${line} ]]
    do
        arrLine=($line)
        if [ "${arrLine[0]}" == "total:" ];then
            continue
        fi
        echo "<tr align=\"center\" ><td>$dirName${arrLine[0]#*$dirName}</td><td>${arrLine[1]}</td><td>${arrLine[2]}</td></tr>" >> $gHTMLFile
    done  < coverage.txt

    echo "</table></body></html>" >> $gHTMLFile
    rm coverage.txt
}

makeTest
genHtml
