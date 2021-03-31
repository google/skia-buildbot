import './index';
import { ExpandableTextareaSk } from './expandable-textarea-sk';
import '../theme-chooser-sk';

const expandableTextareaSk = new ExpandableTextareaSk();
expandableTextareaSk.open = false;
expandableTextareaSk.placeholder = 'Your text here';
expandableTextareaSk.displayText = 'Toggle this textbox';
document.body.querySelector('#container')!.appendChild(expandableTextareaSk);
