module.exports = function(grunt) {
  // Project configuration.
  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    // Install all the packages listed in the bower.json file.
    shell: {
      bower_install: {
         command: './node_modules/.bin/bower install'
      },
      install_npo: {
         command: 'npm install',
         options: {
           execOptions: {
             cwd: 'third_party/bower_components/native-promise-only/'
           }
         }
      },
      build_npo: {
         command: 'node build.js',
         options: {
           execOptions: {
             cwd: 'third_party/bower_components/native-promise-only/'
           }
         }
      }
    },
    // Concatenate all the third_party and common files we use into a single file.
    concat: {
      dist: {
        src: [
          'res/js/common.js',

          'third_party/bower_components/platform/platform.js',

          'third_party/bower_components/jquery/dist/jquery.min.js',
          'third_party/bower_components/flot/jquery.flot.js',
          'third_party/bower_components/flot/jquery.flot.crosshair.js',
          'third_party/bower_components/flot/jquery.flot.navigate.js',
          'third_party/bower_components/flot/jquery.flot.selection.js',

          'third_party/bower_components/native-promise-only/npo.js',
        ],
        dest: 'res/js/<%= pkg.name %>-debug.js'
      }
    },
    // Uglify the one big file into one smaller file.
    uglify: {
      options: {
        banner: '/*! <%= pkg.name %> built <%= grunt.template.today("yyyy-mm-dd") %> */\n'
      },
      build: {
        src: 'res/js/<%= pkg.name %>-debug.js',
        dest: 'res/js/<%= pkg.name %>.js'
      }
    },
    // Simpy copies over the unminimized JS, useful for debugging.
    copy: {
      simple: {
        src: 'res/js/<%= pkg.name %>-debug.js',
        dest: 'res/js/<%= pkg.name %>.js'
      },
      polymer: {
        src: [
          'third_party/bower_components/polymer/layout.html',
          'third_party/bower_components/polymer/polymer.html',
          'third_party/bower_components/polymer/polymer.js'
        ],
        dest: 'res/imp/polymer/',
        expand: true,
        flatten: true
      }
    },
    // Auto prefix any CSS so it works on a wider set of browsers.
    autoprefixer: {
      options: {
        // Task-specific options go here.
      },
      single_file: {
        options: {
          // Target-specific options go here.
        },
        src: 'res/css/main.css',
        dest: 'res/css/<%= pkg.name %>.css'
      },
    },
    karma: {
      unit: {
        configFile: 'karma.conf.js'
      }
    },
    jshint: {
      options: {
        eqeqeq: false,
        eqnull: true,
        sub: true,
        shadow: true,
        reporter: 'lint/reporter.js',
        globals: {
          jQuery: true
        }
      },
      main: [
        'res/js/logic2.js'
      ]
    }

  });

  // Load the plugins for the above commands.
  grunt.loadNpmTasks('grunt-contrib-concat');
  grunt.loadNpmTasks('grunt-contrib-copy');
  grunt.loadNpmTasks('grunt-contrib-cssmin');
  grunt.loadNpmTasks('grunt-contrib-uglify');
  grunt.loadNpmTasks('grunt-shell');
  grunt.loadNpmTasks('grunt-autoprefixer');
  grunt.loadNpmTasks('grunt-karma');
  grunt.loadNpmTasks('grunt-contrib-jshint');

  // By default run all the commands in the right sequence to build our custom
  // minified third_party JS.
  grunt.registerTask('default', ['shell:bower_install', 'shell:install_npo', 'shell:build_npo', 'concat', 'uglify', 'copy:polymer']);

  // A target to build an unminified version, for debugging.
  grunt.registerTask('debug-js', ['shell:bower_install', 'concat', 'copy:simple', 'copy:polymer']);

  // A target to build just the CSS.
  grunt.registerTask('css', ['autoprefixer']);

  // A target to build just the CSS.
  grunt.registerTask('test', ['karma']);
};
