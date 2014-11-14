// Copied from perf and minimally adapted.
// TODO (stephana): Fine tune to needs of correctness.
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

          'third_party/bower_components/webcomponentsjs/webcomponents.js',

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
    copy: {
      // Simply copies over the unminimized JS, useful for debugging.
      simple: {
        src: 'res/js/<%= pkg.name %>-debug.js',
        dest: 'res/js/<%= pkg.name %>.js'
      },
      core: {
        cwd: 'third_party/bower_components',
        src: [
          'core-*/**',
        ],
        dest: 'res/imp/',
        expand: true,
      },
      paper: {
        cwd: 'third_party/bower_components',
        src: [
          'paper-*/**'
        ],
        dest: 'res/imp/',
        expand: true,
      },
      polymer: {
        src: [
          'third_party/bower_components/polymer/**',
        ],
        dest: 'res/imp/polymer/',
        expand: true,
        flatten: true
      }
    },
    mkdir: {
      all: {
        options: {
          create: ['res/vul']
        },
      },
    },
    vulcanize: {
      default: {
        options: {
          csp: true,
          strip: true
        },
        files: {
          'res/vul/elements.html': 'elements.html'
        },
      },
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
  grunt.loadNpmTasks('grunt-autoprefixer');
  grunt.loadNpmTasks('grunt-contrib-concat');
  grunt.loadNpmTasks('grunt-contrib-copy');
  grunt.loadNpmTasks('grunt-contrib-cssmin');
  grunt.loadNpmTasks('grunt-contrib-jshint');
  grunt.loadNpmTasks('grunt-contrib-uglify');
  grunt.loadNpmTasks('grunt-karma');
  grunt.loadNpmTasks('grunt-mkdir');
  grunt.loadNpmTasks('grunt-shell');
  grunt.loadNpmTasks('grunt-vulcanize');

  // By default run all the commands in the right sequence to build our custom
  // minified third_party JS.
  grunt.registerTask('default', ['shell:bower_install']);
};
