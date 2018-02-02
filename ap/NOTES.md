How to copy in the polyfills:

      {
        test: /webcomponentsjs/,
        use: {
          loader: 'file-loader',
          options: {
            name: '[name].[ext]'
          }
        }
      },

