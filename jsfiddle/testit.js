const pickNames = (r, s) => {
  const ret = [];
  let match = [];
  // eslint-disable-next-line no-cond-assign
  while ((match = r.exec(s) || []).length > 0) {
    console.log(+match[1], match[2]);
    ret[+match[1]] = match[2];
  }
  return ret;
};

const code = `  
  #slider1:Foo
  #slider2:Bar
        `;
console.log(pickNames(/#slider(\d):(\S+)/g, code));
