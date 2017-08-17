module.exports = {
  entry: [
    './index'
  ],
  output: {
    path: __dirname + "/dist",
    filename: "bundle.js"
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /(node_modules)/
      }
    ]
  },
  node: {
    console: false,
    process: false,
    setImmediate: false,
  }
}
