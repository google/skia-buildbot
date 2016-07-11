Pdfium Installer
================

The scripts in this directory allow to manage the version of PDFium that is 
being used. 

*build_pdfium.sh* checks out pdfium and builds the commit contained in 
*pdfium-build.commit*. 
After a successful build it will genrate the MD5-hash of the executable and 
upload the executable (with the MD5 hash in it's name) to Google Storage. 
It will also update the *pdfium.md5* file with the new hash. 

*install_pdfium.sh* downloads the pdfium_test executable that corresponds to 
the hash in *pdfium.md5* and saves it in ${GOPATH}/bin under the assumption 
that directory is in the current path. 

To update the version of pdfium_test follow these steps:

- Update the *pdfium-build.commit* file with the desired commit from the 
  [pdfium repository](https://pdfium.googlesource.com/pdfium).
- Run the *build_pdfium.sh* script, which will update the *pdfium.md5* file. 
- Upon success commit the changed files. 
