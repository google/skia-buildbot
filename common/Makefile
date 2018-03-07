build:
	npx webpack

serve:
	npx webpack-dev-server --content-base ./dist --watch

test: build
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

continuous:
	# Setup for continuous testing when ssh'd into a machine.
	# To debug tests, set up port forwarding via ssh with "-L 9876:localhost:9876".
	# Start Xvfb on port :99 so Chrome can start.
	-Xvfb :99 &
	# Continuously monitor the source files and rebuild the test files as needed.
	npx webpack --watch &
	sleep 2
	# Continuously run the tests each time they are modified.
	DISPLAY=:99 npx karma start &

continuous_desktop:
	# Setup for continuous testing when running on the desktop.
	# Continuously monitor the source files and rebuild the test files as needed.
	npx webpack --watch &
	sleep 2
	# Continuously run the tests each time they are modified.
	npx karma start &

docs:
	npx jsdoc -c jsdoc.config.js `find modules -name "*.js"`

