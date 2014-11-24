module.exports = function(grunt) {
  // Project configuration.
  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    // Install all the packages listed in the bower.json file.
    bower: {
      install: {
        options: {
          targetDir: './third_party'
        }
      }
    },
    // Concatenate all the third_party and common files we use into a single file.
    concat: {
      dist: {
        src: [
          'third_party/bower_components/native-promise-only/npo.js',
          'third_party/bower_components/webcomponentsjs/webcomponents.js',

          '../res/js/common.js',

          'third_party/bower_components/jquery/dist/jquery.min.js',
          'third_party/bower_components/flot/jquery.flot.js',
          'third_party/bower_components/flot/jquery.flot.crosshair.js',
          'third_party/bower_components/flot/jquery.flot.navigate.js',
          'third_party/bower_components/flot/jquery.flot.selection.js',
        ],
        dest: 'res/js/core-debug.js'
      }
    },
    // Uglify the one big file into one smaller file.
    uglify: {
      options: {
        banner: '/*! core built <%= grunt.template.today("yyyy-mm-dd") %> */\n'
      },
      build: {
        src: 'res/js/core-debug.js',
        dest: 'res/js/core.js'
      }
    },
    copy: {
      // Simply copies over the unminimized JS, useful for debugging.
      debug_over_non_debug: {
        src: 'res/js/core-debug.js',
        dest: 'res/js/core.js'
      },
      elements_html: {
        src: 'elements.html',
        dest: 'res/vul/elements.html'
      },
      webcomponents: {
        files: [
          {
            cwd: 'third_party/bower_components',
            src: [ 'core-*/**' ],
            dest: 'res/imp/',
            expand: true
          },
          {
            cwd: 'third_party/bower_components',
            src: [ 'paper-*/**' ],
            dest: 'res/imp/',
            expand: true
          },
          {
            src: [ 'third_party/bower_components/polymer/**', ],
            dest: 'res/imp/polymer/',
            expand: true,
            flatten: true
          }
        ]
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
          csp: false,
          inline: true,
          strip: true,
          abspath: './'

        },
        files: {
          'res/vul/elements.html': 'elements.html'
        },
      },
    },
    // Auto prefix any CSS so it works on a wider set of browsers.
    autoprefixer: {
      single_file: {
        src: 'res/css/main.css',
        dest: 'res/css/perf.css'
      },
    },
    karma: {
      unit: {
        configFile: 'karma.conf.js'
      }
    }
  });

  // Load the plugins for the above commands.
  grunt.loadNpmTasks('grunt-autoprefixer');
  grunt.loadNpmTasks('grunt-contrib-concat');
  grunt.loadNpmTasks('grunt-contrib-copy');
  grunt.loadNpmTasks('grunt-contrib-cssmin');
  grunt.loadNpmTasks('grunt-contrib-uglify');
  grunt.loadNpmTasks('grunt-karma');
  grunt.loadNpmTasks('grunt-mkdir');
  grunt.loadNpmTasks('grunt-shell');
  grunt.loadNpmTasks('grunt-vulcanize');
  grunt.loadNpmTasks('grunt-bower-task');

  // By default run all the commands in the right sequence to build our custom
  // minified third_party JS.
  grunt.registerTask('default',
    ['bower:install', 'concat', 'uglify', 'copy:webcomponents', 'mkdir', 'vulcanize']);

  // A target to build an unminified version, for debugging.
  grunt.registerTask('debug',
    ['bower:install', 'concat', 'copy:debug_over_non_debug', 'copy:webcomponents', 'copy:elements_html']);

  // A target to build just the CSS.
  grunt.registerTask('css', ['autoprefixer']);

  // A target to build just the CSS.
  grunt.registerTask('test', ['karma']);
};
