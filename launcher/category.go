// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package launcher

import (
	"fmt"
	"sort"
	"strings"
)

type CategoryID int64

const (
	CategoryInternet CategoryID = iota
	CategoryChat
	CategoryMusic
	CategoryVideo
	CategoryGraphics
	CategoryGame
	CategoryOffice
	CategoryReading
	CategoryDevelopment
	CategorySystem
	CategoryOthers
)

func (cid CategoryID) String() string {
	var str string
	switch cid {
	case CategoryInternet:
		str = "Internet"
	case CategoryChat:
		str = "Chat"
	case CategoryMusic:
		str = "Music"
	case CategoryVideo:
		str = "Video"
	case CategoryGraphics:
		str = "Graphics"
	case CategoryOffice:
		str = "Office"
	case CategoryGame:
		str = "Game"
	case CategoryReading:
		str = "Reading"
	case CategoryDevelopment:
		str = "Development"
	case CategorySystem:
		str = "System"
	case CategoryOthers:
		str = "Others"
	default:
		str = "Unknown"
	}
	return str
}

func (cid CategoryID) Pinyin() string {
	switch cid {
	case CategoryInternet:
		return "wangluo"
	case CategoryChat:
		return "shejiaogoutong"
	case CategoryMusic:
		return "yinyuexinshang"
	case CategoryVideo:
		return "shipinbofang"
	case CategoryGraphics:
		return "tuxintuxiang"
	case CategoryOffice:
		return "bangongxuexi"
	case CategoryGame:
		return "youxiyule"
	case CategoryReading:
		return "yuedufanyi"
	case CategoryDevelopment:
		return "bianchengkaifai"
	case CategorySystem:
		return "xitongguanli"
	case CategoryOthers:
		return "qita"
	default:
		return "qita"
	}
}

var categoryNameTable = map[string]CategoryID{
	"internet":    CategoryInternet,
	"chat":        CategoryChat,
	"music":       CategoryMusic,
	"video":       CategoryVideo,
	"graphics":    CategoryGraphics,
	"office":      CategoryOffice,
	"game":        CategoryGame,
	"reading":     CategoryReading,
	"development": CategoryDevelopment,
	"system":      CategorySystem,
	"others":      CategoryOthers,
}

func parseCategoryString(str string) (CategoryID, bool) {
	if str == "" {
		return CategoryOthers, false
	}

	cid, ok := categoryNameTable[str]
	if !ok {
		return CategoryOthers, false
	}
	return cid, true
}

var xCategories = map[string][]CategoryID{
	"2dgraphics":                      {CategoryGraphics},
	"3dgraphics":                      {CategoryGraphics},
	"accessibility":                   {CategorySystem},
	"accessories":                     {CategoryOthers},
	"actiongame":                      {CategoryGame},
	"advancedsettings":                {CategorySystem},
	"adventuregame":                   {CategoryGame},
	"amusement":                       {CategoryGame},
	"applet":                          {CategoryOthers},
	"arcadegame":                      {CategoryGame},
	"archiving":                       {CategorySystem},
	"art":                             {CategoryOffice},
	"artificialintelligence":          {CategoryOffice},
	"astronomy":                       {CategoryOffice},
	"audio":                           {CategoryMusic},
	"audiovideo":                      {CategoryMusic, CategoryVideo},
	"audiovideoediting":               {CategoryMusic, CategoryVideo},
	"biology":                         {CategoryOffice},
	"blocksgame":                      {CategoryGame},
	"boardgame":                       {CategoryGame},
	"building":                        {CategoryDevelopment},
	"calculator":                      {CategorySystem},
	"calendar":                        {CategorySystem},
	"cardgame":                        {CategoryGame},
	"cd":                              {CategoryMusic},
	"chart":                           {CategoryOffice},
	"chat":                            {CategoryChat},
	"chemistry":                       {CategoryOffice},
	"clock":                           {CategorySystem},
	"compiz":                          {CategorySystem},
	"compression":                     {CategorySystem},
	"computerscience":                 {CategoryOffice},
	"consoleonly":                     {CategoryOthers},
	"contactmanagement":               {CategoryChat},
	"core":                            {CategoryOthers},
	"debugger":                        {CategoryDevelopment},
	"desktopsettings":                 {CategorySystem},
	"desktoputility":                  {CategorySystem},
	"development":                     {CategoryDevelopment},
	"dialup":                          {CategorySystem},
	"dictionary":                      {CategoryOffice},
	"discburning":                     {CategorySystem},
	"documentation":                   {CategoryOffice},
	"editors":                         {CategoryOthers},
	"education":                       {CategoryOffice},
	"electricity":                     {CategoryOffice},
	"electronics":                     {CategoryOffice},
	"email":                           {CategoryInternet},
	"emulator":                        {CategoryGame},
	"engineering":                     {CategorySystem},
	"favorites":                       {CategoryOthers},
	"filemanager":                     {CategorySystem},
	"filesystem":                      {CategorySystem},
	"filetools":                       {CategorySystem},
	"filetransfer":                    {CategoryInternet},
	"finance":                         {CategoryOffice},
	"game":                            {CategoryGame},
	"geography":                       {CategoryOffice},
	"geology":                         {CategoryOffice},
	"geoscience":                      {CategoryOthers},
	"gnome":                           {CategorySystem},
	"gpe":                             {CategoryOthers},
	"graphics":                        {CategoryGraphics},
	"guidesigner":                     {CategoryDevelopment},
	"hamradio":                        {CategoryOffice},
	"hardwaresettings":                {CategorySystem},
	"ide":                             {CategoryDevelopment},
	"imageprocessing":                 {CategoryGraphics},
	"instantmessaging":                {CategoryChat},
	"internet":                        {CategoryInternet},
	"ircclient":                       {CategoryChat},
	"kde":                             {CategorySystem},
	"kidsgame":                        {CategoryGame},
	"literature":                      {CategoryOffice},
	"logicgame":                       {CategoryGame},
	"math":                            {CategoryOffice},
	"medicalsoftware":                 {CategoryOffice},
	"meteorology":                     {CategoryOthers},
	"midi":                            {CategoryMusic},
	"mixer":                           {CategoryMusic},
	"monitor":                         {CategorySystem},
	"motif":                           {CategoryOthers},
	"multimedia":                      {CategoryVideo},
	"music":                           {CategoryMusic},
	"network":                         {CategoryInternet},
	"news":                            {CategoryReading},
	"numericalanalysis":               {CategoryOffice},
	"ocr":                             {CategoryGraphics},
	"office":                          {CategoryOffice},
	"p2p":                             {CategoryInternet},
	"packagemanager":                  {CategorySystem},
	"panel":                           {CategorySystem},
	"pda":                             {CategorySystem},
	"photography":                     {CategoryGraphics},
	"physics":                         {CategoryOffice},
	"pim":                             {CategoryOthers},
	"player":                          {CategoryMusic, CategoryVideo},
	"playonlinux":                     {CategoryOthers},
	"presentation":                    {CategoryOffice},
	"printing":                        {CategoryOffice},
	"profiling":                       {CategoryDevelopment},
	"projectmanagement":               {CategoryOffice},
	"publishing":                      {CategoryOffice},
	"puzzlegame":                      {CategoryGame},
	"rastergraphics":                  {CategoryGraphics},
	"recorder":                        {CategoryMusic, CategoryVideo},
	"remoteaccess":                    {CategorySystem},
	"revisioncontrol":                 {CategoryDevelopment},
	"robotics":                        {CategoryOffice},
	"roleplaying":                     {CategoryGame},
	"scanning":                        {CategoryOffice},
	"science":                         {CategoryOffice},
	"screensaver":                     {CategoryOthers},
	"sequencer":                       {CategoryMusic},
	"settings":                        {CategorySystem},
	"security":                        {CategorySystem},
	"simulation":                      {CategoryGame},
	"sportsgame":                      {CategoryGame},
	"spreadsheet":                     {CategoryOffice},
	"strategygame":                    {CategoryGame},
	"system":                          {CategorySystem},
	"systemsettings":                  {CategorySystem},
	"technical":                       {CategoryOthers},
	"telephony":                       {CategorySystem},
	"telephonytools":                  {CategorySystem},
	"terminalemulator":                {CategorySystem},
	"texteditor":                      {CategoryOffice},
	"texttools":                       {CategoryOffice},
	"transiation":                     {CategoryDevelopment},
	"translation":                     {CategoryReading},
	"trayicon":                        {CategorySystem},
	"tuner":                           {CategoryMusic},
	"tv":                              {CategoryVideo},
	"utility":                         {CategorySystem},
	"vectorgraphics":                  {CategoryGraphics},
	"video":                           {CategoryVideo},
	"videoconference":                 {CategoryInternet},
	"viewer":                          {CategoryGraphics},
	"webbrowser":                      {CategoryInternet},
	"webdevelopment":                  {CategoryDevelopment},
	"wine":                            {CategoryOthers},
	"wine-programs-accessories":       {CategoryOthers},
	"wordprocessor":                   {CategoryOffice},
	"x-alsa":                          {CategoryMusic},
	"x-bible":                         {CategoryReading},
	"x-bluetooth":                     {CategorySystem},
	"x-debian-applications-emulators": {CategoryGame},
	"x-digital_processing":            {CategorySystem},
	"x-enlightenment":                 {CategorySystem},
	"x-geeqie":                        {CategoryGraphics},
	"x-gnome-networksettings":         {CategorySystem},
	"x-gnome-personalsettings":        {CategorySystem},
	"x-gnome-settings-panel":          {CategorySystem},
	"x-gnome-systemsettings":          {CategorySystem},
	"x-gnustep":                       {CategorySystem},
	"x-islamic-software":              {CategoryReading},
	"x-jack":                          {CategoryMusic},
	"x-kde-edu-misc":                  {CategoryReading},
	"x-kde-internet":                  {CategorySystem},
	"x-kde-more":                      {CategorySystem},
	"x-kde-utilities-desktop":         {CategorySystem},
	"x-kde-utilities-file":            {CategorySystem},
	"x-kde-utilities-peripherals":     {CategorySystem},
	"x-kde-utilities-pim":             {CategorySystem},
	"x-lxde-settings":                 {CategorySystem},
	"x-mandriva-office-publishing":    {CategoryOthers},
	"x-mandrivalinux-internet-other":  {CategorySystem},
	"x-mandrivalinux-office-other":    {CategoryOffice},
	"x-mandrivalinux-system-archiving-backup": {CategorySystem},
	"x-midi":                           {CategoryMusic},
	"x-misc":                           {CategorySystem},
	"x-multitrack":                     {CategoryMusic},
	"x-novell-main":                    {CategorySystem},
	"x-quran":                          {CategoryReading},
	"x-red-hat-base":                   {CategorySystem},
	"x-red-hat-base-only":              {CategorySystem},
	"x-red-hat-extra":                  {CategorySystem},
	"x-red-hat-serverconfig":           {CategorySystem},
	"x-religion":                       {CategoryReading},
	"x-sequencers":                     {CategoryMusic},
	"x-sound":                          {CategoryMusic},
	"x-sun-supported":                  {CategorySystem},
	"x-suse-backup":                    {CategorySystem},
	"x-suse-controlcenter-lookandfeel": {CategorySystem},
	"x-suse-controlcenter-system":      {CategorySystem},
	"x-suse-core":                      {CategorySystem},
	"x-suse-core-game":                 {CategoryGame},
	"x-suse-core-office":               {CategoryOffice},
	"x-suse-sequencer":                 {CategoryMusic},
	"x-suse-yast":                      {CategorySystem},
	"x-suse-yast-high_availability":    {CategorySystem},
	"x-synthesis":                      {CategorySystem},
	"x-turbolinux-office":              {CategoryOffice},
	"x-xfce":                           {CategorySystem},
	"x-xfce-toplevel":                  {CategorySystem},
	"x-xfcesettingsdialog":             {CategorySystem},
	"x-ximian-main":                    {CategorySystem},
}

func parseXCategoryString(name string) []CategoryID {
	name = strings.ToLower(name)
	id, ok := categoryNameTable[name]
	if ok {
		return []CategoryID{id}
	}

	if ids, ok := xCategories[name]; ok {
		return ids
	}
	logger.Debugf("parseXCategoryString unknown category %q", name)
	return []CategoryID{CategoryOthers}
}

func getXCategory(categories []string) CategoryID {
	categoriesCountMap := make(map[CategoryID]int)
	for _, categoryStr := range categories {
		ids := parseXCategoryString(categoryStr)
		for _, id := range ids {
			categoriesCountMap[id]++
		}
	}

	// ignore CategoryOthers
	delete(categoriesCountMap, CategoryOthers)
	logger.Debug("getXCategory categoriesCountMap:", categoriesCountMap)

	if len(categoriesCountMap) == 0 {
		return CategoryOthers
	}

	var categoryCountMax int
	maxCategory := CategoryOthers
	for cid, count := range categoriesCountMap {
		if count > categoryCountMax {
			categoryCountMax = count
			maxCategory = cid
		}
	}

	maxCategories := []CategoryID{maxCategory}
	for cid, count := range categoriesCountMap {
		if count == categoryCountMax && cid != maxCategory {
			maxCategories = append(maxCategories, cid)
		}
	}
	if len(maxCategories) == 1 {
		return maxCategories[0]
	}

	sort.Slice(maxCategories, func(i, j int) bool {
		return maxCategories[i] < maxCategories[j]
	})
	maxCategoriesStr := fmt.Sprintf("%v", maxCategories)
	logger.Debug("maxCategories:", maxCategoriesStr)
	if strings.Contains(maxCategoriesStr, "Music Video") {
		return CategoryVideo
	}
	return maxCategories[0]
}
