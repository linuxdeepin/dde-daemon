PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = github.com/linuxdeepin/dde-daemon
GOBUILD = go build $(GO_BUILD_FLAGS)
export GO111MODULE=off
export GOPATH=$(shell go env GOPATH)

BINARIES =  \
	    dde-session-daemon \
	    dde-system-daemon \
	    grub2 \
	    search \
	    theme-thumb-tool \
	    backlight_helper \
	    langselector \
	    soundeffect \
	    dde-lockservice \
	    dde-authority \
	    default-terminal \
	    dde-greeter-setter

LANGUAGES = $(basename $(notdir $(wildcard misc/po/*.po)))

CFLAGS = -W -Wall -fPIC -fstack-protector-all -z relro -z noexecstack -z now -pie

all: build

prepare:
	@mkdir -p out/bin
	@mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX});
	@ln -snf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX};

out/bin/%: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o $@ ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/bin/${@F}

out/bin/default-file-manager: bin/default-file-manager/main.c
	gcc $^ $(shell pkg-config --cflags --libs gio-unix-2.0) $(CFLAGS) -o $@

out/bin/desktop-toggle: bin/desktop-toggle/main.c
	gcc $^ $(shell pkg-config --cflags --libs x11) $(CFLAGS) -o $@

out/locale/%/LC_MESSAGES/dde-daemon.mo: misc/po/%.po
	mkdir -p $(@D)
	msgfmt -o $@ $<

translate: $(addsuffix /LC_MESSAGES/dde-daemon.mo, $(addprefix out/locale/, ${LANGUAGES}))

pot:
	deepin-update-pot misc/po/locale_config.ini

POLICIES=accounts Grub2 Fprintd
ts:
	for i in $(POLICIES); do \
		deepin-policy-ts-convert policy2ts misc/polkit-action/com.deepin.daemon.$$i.policy.in misc/ts/com.deepin.daemon.$$i.policy; \
	done

ts_to_policy:
	for i in $(POLICIES); do \
	deepin-policy-ts-convert ts2policy misc/polkit-action/com.deepin.daemon.$$i.policy.in misc/ts/com.deepin.daemon.$$i.policy misc/polkit-action/com.deepin.daemon.$$i.policy; \
	done

build: prepare out/bin/default-terminal out/bin/default-file-manager out/bin/desktop-toggle $(addprefix out/bin/, ${BINARIES}) ts_to_policy icons translate

test: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go test -v ./...

test-coverage: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go test -cover -v ./... | awk '$$2 ~ "_${CURDIR}" {print $$2","$$5}' | sed "s:${CURDIR}::g" | sed 's/files\]/0\.0%/g' > coverage.csv

print_gopath: prepare
	GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}"

install: build install-dde-data install-icons
	mkdir -pv ${DESTDIR}${PREFIX}/lib/deepin-daemon
	cp -f out/bin/* ${DESTDIR}${PREFIX}/lib/deepin-daemon/

	mkdir -pv ${DESTDIR}${PREFIX}/share/locale
	cp -r out/locale/* ${DESTDIR}${PREFIX}/share/locale

	mkdir -pv ${DESTDIR}${PREFIX}/share/dbus-1/system.d
	cp misc/conf/*.conf ${DESTDIR}${PREFIX}/share/dbus-1/system.d/

	mkdir -pv ${DESTDIR}${PREFIX}/share/dbus-1
	cp -r misc/services ${DESTDIR}${PREFIX}/share/dbus-1/
	cp -r misc/system-services ${DESTDIR}${PREFIX}/share/dbus-1/

	mkdir -pv ${DESTDIR}${PREFIX}/share/polkit-1/actions
	cp misc/polkit-action/*.policy ${DESTDIR}${PREFIX}/share/polkit-1/actions/

	mkdir -pv ${DESTDIR}/var/lib/polkit-1/localauthority/10-vendor.d
	cp misc/polkit-localauthority/*.pkla ${DESTDIR}/var/lib/polkit-1/localauthority/10-vendor.d/

	mkdir -pv ${DESTDIR}${PREFIX}/share/dde-daemon
	cp -r misc/dde-daemon/*   ${DESTDIR}${PREFIX}/share/dde-daemon/
	cp -r misc/usr/share/deepin ${DESTDIR}${PREFIX}/share/

	mkdir -pv ${DESTDIR}/lib/systemd/system/
	cp -f misc/systemd/services/* ${DESTDIR}/lib/systemd/system/

	mkdir -pv ${DESTDIR}/etc/pam.d/
	cp -f misc/etc/pam.d/* ${DESTDIR}/etc/pam.d/

	mkdir -pv ${DESTDIR}/etc/default/grub.d
	cp -f misc/etc/default/grub.d/* ${DESTDIR}/etc/default/grub.d

	mkdir -pv ${DESTDIR}/etc/deepin
	cp -f misc/etc/deepin/* ${DESTDIR}/etc/deepin

	mkdir -pv ${DESTDIR}/etc/acpi/events
	cp -f misc/etc/acpi/events/* ${DESTDIR}/etc/acpi/events/

	mkdir -pv ${DESTDIR}/etc/acpi/actions
	cp -f misc/etc/acpi/actions/* ${DESTDIR}/etc/acpi/actions/

	mkdir -pv ${DESTDIR}/etc/pulse/daemon.conf.d
	cp -f misc/etc/pulse/daemon.conf.d/*.conf ${DESTDIR}/etc/pulse/daemon.conf.d/

	mkdir -pv ${DESTDIR}/lib/udev/rules.d
	cp -f misc/udev-rules/*.rules ${DESTDIR}/lib/udev/rules.d/

	mkdir -pv ${DESTDIR}/usr/lib/deepin-daemon/service-trigger
	cp -f misc/service-trigger/*.json ${DESTDIR}/usr/lib/deepin-daemon/service-trigger/
	cp -f misc/service-trigger/*.sh ${DESTDIR}/usr/lib/deepin-daemon/service-trigger/

	mkdir -pv ${DESTDIR}/etc/NetworkManager/conf.d
	cp -f misc/etc/NetworkManager/conf.d/* ${DESTDIR}/etc/NetworkManager/conf.d/

	mkdir -pv ${DESTDIR}${PREFIX}/libexec/dde-daemon/
	cp -r misc/libexec/dde-daemon/* ${DESTDIR}${PREFIX}/libexec/dde-daemon/

	mkdir -pv ${DESTDIR}${PREFIX}/share/dsg/configs/org.deepin.dde.daemon/
	cp -r misc/dsg-configs/*.json ${DESTDIR}${PREFIX}/share/dsg/configs/org.deepin.dde.daemon/
	mkdir -pv ${DESTDIR}${PREFIX}/share/dsg/configs/org.deepin.dde.lightdm-deepin-greeter
	cp -r misc/dsg-configs/org.deepin.dde.lightdm-deepin-greeter/*.json ${DESTDIR}${PREFIX}/share/dsg/configs/org.deepin.dde.lightdm-deepin-greeter/

install-dde-data:
	mkdir -pv ${DESTDIR}${PREFIX}/share/dde/
	cp -r misc/data ${DESTDIR}${PREFIX}/share/dde/

icons:
	python3 misc/icons/install_to_hicolor.py -d status -o out/icons misc/icons/status

install-icons: icons
	mkdir -pv ${DESTDIR}${PREFIX}/share/icons/
	cp -r out/icons/hicolor ${DESTDIR}${PREFIX}/share/icons/

clean:
	rm -rf ${GOPATH_DIR}
	rm -rf out

rebuild: clean build

check_code_quality: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go vet ./...
