all: 
	@./build.sh
clean:
	@rm -f jd
install: all
	@cp jd /usr/local/bin
uninstall: 
	@rm -f /usr/local/bin/jd
package:
	@NOCOPY=1 ./build.sh package
