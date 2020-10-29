import { BranchesSk } from './branches-sk';
import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { complexBranchData, doubleBranchData, singleBranchData } from './test_data';

const single = document.querySelector('#single') as BranchesSk;
[single.branchHeads, single.commits] = singleBranchData;
const double = document.querySelector('#double') as BranchesSk;
[double.branchHeads, double.commits] = doubleBranchData;
const complex = document.querySelector('#complex') as BranchesSk;
[complex.branchHeads, complex.commits] = complexBranchData;
