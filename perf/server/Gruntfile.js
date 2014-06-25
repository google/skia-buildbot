module.exports = function(grunt) {
  // Project configuration.
  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    // Install all the packages listed in the bower.json file.
    shell: {
      bower_install: {
         command: 'bower install'
      }
    },
    // Concatenate all the third_party files we use into a single file.
    concat: {
      dist: {
        src: [
          'third_party/bower_components/jquery/dist/jquery.js',
          'third_party/bower_components/flot/jquery.flot.js',
          'third_party/bower_components/flot/jquery.flot.crosshair.js',
          'third_party/bower_components/flot/jquery.flot.navigate.js'
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
  });

  // Load the plugins for the above commands.
  grunt.loadNpmTasks('grunt-bower');
  grunt.loadNpmTasks('grunt-contrib-concat');
  grunt.loadNpmTasks('grunt-contrib-copy');
  grunt.loadNpmTasks('grunt-contrib-cssmin');
  grunt.loadNpmTasks('grunt-contrib-uglify');
  grunt.loadNpmTasks('grunt-shell');
  grunt.loadNpmTasks('grunt-autoprefixer');

  // By default run all the commands in the right sequence to build our custom
  // minified third_party JS.
  grunt.registerTask('default', ['shell:bower_install', 'concat', 'uglify']);

  // A target to build an unminified version, for debugging.
  grunt.registerTask('debug-js', ['shell:bower_install', 'concat', 'copy:simple']);

  // A target to build just the CSS.
  grunt.registerTask('css', ['autoprefixer']);
};
