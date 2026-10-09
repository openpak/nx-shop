package main

// Page assets the shop shares with nx-baas's own pages, copied rather than shared: they are
// page content that may diverge per page. logoDataURI and pageCSS are link.go's, footerJS is
// nso.go's (the console's own button bar, through window.nx.footer).

const logoDataURI = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA1MTIgNTEyIiBmaWxsPSJub25lIj4gPGRlZnM+IDxsaW5lYXJHcmFkaWVudCBpZD0iYmciIHgxPSIwIiB5MT0iMCIgeDI9IjUxMiIgeTI9IjUxMiIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPiA8c3RvcCBvZmZzZXQ9IjAlIiBzdG9wLWNvbG9yPSIjNjM2NmYxIi8+IDxzdG9wIG9mZnNldD0iMTAwJSIgc3RvcC1jb2xvcj0iIzQzMzhjYSIvPiA8L2xpbmVhckdyYWRpZW50PiA8L2RlZnM+IDxjaXJjbGUgY3g9IjI1NiIgY3k9IjI1NiIgcj0iMjU2IiBmaWxsPSJ1cmwoI2JnKSIvPiA8Y2lyY2xlIGN4PSIyNTYiIGN5PSIyNTYiIHI9IjE4MCIgc3Ryb2tlPSJ3aGl0ZSIgc3Ryb2tlLXdpZHRoPSIyMCIgZmlsbD0ibm9uZSIgb3BhY2l0eT0iMC45Ii8+IDxwYXRoIGQ9Ik0xNjAgMjU2YzAtNTMgNDMtOTYgOTYtOTZzOTYgNDMgOTYgOTYtNDMgOTYtOTYgOTYiIHN0cm9rZT0id2hpdGUiIHN0cm9rZS13aWR0aD0iMjAiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgZmlsbD0ibm9uZSIgb3BhY2l0eT0iMC45Ii8+IDxjaXJjbGUgY3g9IjI1NiIgY3k9IjI1NiIgcj0iMjgiIGZpbGw9IndoaXRlIiBvcGFjaXR5PSIwLjk1Ii8+IDwvc3ZnPg=="

const pageCSS = `
/* The site's tokens (website/web/src/input.css, 2026-10 "console HOME menu" redesign), so the
   page a console sends you to looks like the place it sends you to: a cool grey shell, white
   tiles, slate ink, indigo for links and primary actions, and a cyan selection cursor that
   appears only on hover, focus and the selected thing. Flat colours, no gradients. Light and
   dark follow the reader's system setting: this opens on a phone in daylight as often as on a
   television at night. Inline on purpose — a console browser may have no route to the website. */
:root{color-scheme:light dark;--paper:#e9ecf0;--paper-2:#f2f4f7;--raised:#ffffff;--sunk:#dde1e7;--ink:#20262f;--ink-2:#48505c;--muted:#6b7380;--faint:#9aa1ab;
--line:#d9dde3;--line-2:#c3c9d1;--cursor:#0094be;--cursor-2:#39c9ea;
--accent:#4f46e5;--accent-2:#6366f1;--accent-weak:#eceeff;--accent-line:#c5cafb;--on-accent:#fff;
--ok:#138a45;--ok-weak:#e3f5ea;--ok-line:#b2e0c3;--bad:#c42b2b;--bad-weak:#fde9e9;--bad-line:#f2bfbf;
--warn:#a35f00;--warn-weak:#fdf2dc;--warn-line:#efd39c;
--shadow-sm:0 1px 0 rgba(32,38,47,.06);--shadow:0 1px 0 rgba(32,38,47,.06),0 6px 16px -10px rgba(32,38,47,.25);--r:14px}
@media (prefers-color-scheme:dark){:root{--paper:#24272d;--paper-2:#2b2f36;--raised:#31353d;--sunk:#1d2025;--ink:#eef1f5;--ink-2:#c4c9d1;--muted:#949ba6;--faint:#6c737e;
--line:#3b4048;--line-2:#4a505a;--cursor:#3ad7f2;--cursor-2:#a2efff;
--accent:#9198ff;--accent-2:#a5abff;--accent-weak:#2e3156;--accent-line:#4a4f8c;--on-accent:#15172e;
--ok:#4fd486;--ok-weak:#1f3a2b;--ok-line:#2f5c42;--bad:#ff7676;--bad-weak:#432528;--bad-line:#6e3439;
--warn:#f6bd4f;--warn-weak:#3f3320;--warn-line:#6a5328;
--shadow-sm:0 1px 0 rgba(0,0,0,.25);--shadow:0 1px 0 rgba(0,0,0,.25),0 8px 18px -10px rgba(0,0,0,.6)}}
*{box-sizing:border-box}
body{margin:0;background:var(--paper);color:var(--ink);
font:16px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;
display:flex;justify-content:center;padding:24px 16px 40px}
main{width:100%;max-width:34rem}
h1{font-size:clamp(1.5rem,5vw,1.8rem);line-height:1.2;letter-spacing:-0.02em;margin:0 0 6px;font-weight:650}
p{color:var(--muted);margin:0 0 20px}
strong{color:var(--ink)}
.brand{display:flex;align-items:center;gap:10px;font-weight:700;margin-bottom:24px;color:var(--ink)}
.brand img{width:28px;height:28px}
.card{--ring-gap:var(--raised);background:var(--raised);border:1px solid var(--line);border-radius:var(--r);padding:20px;margin-bottom:16px;box-shadow:var(--shadow-sm)}
/* The code is the whole point of the screen it appears on: as large as it can
   be without wrapping on a narrow phone. */
.code{font:600 clamp(1.8rem,9vw,2.4rem)/1 ui-monospace,SFMono-Regular,Menlo,monospace;
letter-spacing:0.16em;color:var(--accent);background:var(--accent-weak);
border:1px solid var(--accent-line);border-radius:10px;padding:16px 18px;display:inline-block}
.qr{background:#fff;padding:12px;border-radius:10px;display:inline-block;line-height:0}
.qr img{width:min(220px,60vw);height:auto;aspect-ratio:1}
.split{display:flex;gap:20px;flex-wrap:wrap;align-items:center}
label{display:block;font-size:0.92rem;color:var(--ink-2);margin:14px 0 6px}
/* 44px tall and never under 16px: a smaller field makes iOS zoom on focus and
   leaves the page scrolled sideways. */
input{width:100%;min-height:44px;padding:0.65rem 0.8rem;font:inherit;font-size:max(1rem,16px);
color:var(--ink);background:var(--paper-2);border:1px solid var(--line-2);border-radius:12px}
button{margin-top:18px;min-height:44px;padding:0.7rem 1.1rem;font:inherit;font-weight:600;
color:var(--on-accent);background:var(--accent);border:1px solid var(--accent);border-radius:999px;
cursor:pointer;width:100%}
button.ghost{background:transparent;border-color:var(--line-2);color:var(--ink);margin-top:10px}
/* The selection cursor: a gap in the page's own colour, then the cyan ring. A D-pad moves focus,
   so :focus is what the player sees as the cursor; hover selects for a pointer. Inside a card the
   gap is the card's colour (--ring-gap). It breathes between two cyans where motion is welcome. */
a:focus,button:hover,button:focus,summary:focus,input:focus{outline:none;box-shadow:0 0 0 3px var(--ring-gap,var(--paper)),0 0 0 7px var(--cursor)}
a:focus,summary:focus{border-radius:8px}
input:focus{border-color:var(--cursor);background:var(--raised)}
@keyframes breathe{50%{box-shadow:0 0 0 3px var(--ring-gap,var(--paper)),0 0 0 7px var(--cursor-2)}}
@media (prefers-reduced-motion:no-preference){a:focus,button:hover,button:focus,summary:focus{animation:breathe 1.6s ease-in-out infinite}}
@media (prefers-color-scheme:dark){.card{border-color:transparent}}
/* One row, tinted by state, with a pill on the right — the same shape the
   status page uses, so the two read as one product. */
.row{display:flex;align-items:center;gap:0.75rem;padding:0.9rem 1rem;border-radius:12px;
border:1px solid var(--line);background:var(--raised);margin-bottom:0.6rem}
.row .name{font-weight:600;overflow-wrap:anywhere}
.row .pill{margin-left:auto;flex:none;white-space:nowrap;display:inline-flex;align-items:center;
gap:0.4rem;padding:0.3rem 0.7rem;border-radius:999px;font-size:0.85rem;font-weight:600;
background:transparent;border:1px solid var(--line-2);color:var(--muted)}
.row .pill::before{content:"";width:0.45rem;height:0.45rem;border-radius:50%;background:currentColor}
.row.ok{background:var(--ok-weak);border-color:var(--ok-line)}
.row.ok .pill{border-color:var(--ok-line);color:var(--ok)}
.row.bad{background:var(--bad-weak);border-color:var(--bad-line)}
.row.bad .pill{border-color:var(--bad-line);color:var(--bad)}
.err{color:var(--bad);margin:0 0 12px}
.ok{color:var(--ok)}
.steps{margin:0;padding-left:1.2em;color:var(--muted)}
.steps li{margin:6px 0}
hr{border:0;border-top:1px solid var(--line);margin:24px 0}
details summary{color:var(--muted);cursor:pointer;min-height:44px;display:flex;align-items:center}

/* The console browser: a television, read from a sofa, driven with a stick.
   Everything is bigger, the two halves sit side by side because a 720p screen
   is wide and short, and focus is visible because there is no pointer to tell
   you where you are. Televisions also crop their edges, so nothing important
   goes near them. */
body.console{font-size:20px;padding:40px 56px;align-items:flex-start}
.console main{max-width:66rem}
.console h1{font-size:2.5rem;margin-bottom:10px}
.console p{font-size:1.05rem}
.console .brand{font-size:1.3rem;margin-bottom:28px}
.console .brand img{width:36px;height:36px}
.console .card{padding:28px}
.console .split{flex-wrap:nowrap;gap:40px;align-items:flex-start}
.console .qr img{width:300px}
/* The code is read across a room: it is the largest thing on the screen. */
.console .code{font-size:3.2rem;letter-spacing:0.2em;padding:20px 26px}
.console .steps{font-size:1.05rem;line-height:1.7}
.console .steps li{margin:10px 0}
.console button{width:auto;min-height:60px;padding:0.85rem 1.8rem;font-size:1.05rem;margin-top:0}
.console #confirm{display:flex;gap:16px;margin-top:22px}
.console #confirm[hidden]{display:none}
.console label{font-size:1rem}
.console input{min-height:52px;font-size:1.05rem}
/* No pointer: the cursor ring above announces the focused control. */
@media (max-width:900px){
  .console .split{flex-wrap:wrap;gap:24px}
  body.console{padding:24px 20px;font-size:18px}
  .console .qr img{width:min(240px,60vw)}
}
`

// footerJS drives the console's own button bar through window.nx.footer (the web applet's
// footer API: setAssign(button, label, callback) and unsetAssign(button)). Only Confirm and
// Back are shown, as on Nintendo's pages: the browser's End (X) and zoom (R/L) assignments are
// removed, and L/R are given to the page when it wants them. Nothing here is confirmed on
// 22.5.0 hardware, so what the footer object offers and every call's outcome are reported.
const footerJS = `
function nxFooterSetup(handlers,rep){
 var f=window.nx&&window.nx.footer;
 var out={event:'footer',present:!!f,keys:[],calls:{}};
 if(!f){rep(out);return;}
 try{for(var k in f){out.keys.push(k);}}catch(e){}
 function call(name,args){try{var r=f[name].apply(f,args);out.calls[name+'('+args[0]+')']=String(r);}catch(e){out.calls[name+'('+args[0]+')']='!'+String(e);}}
 var drop=['X','Y','L','R','ZL','ZR','Plus','Minus'];
 for(var i=0;i<drop.length;i++){if(!handlers[drop[i]]&&typeof(f.unsetAssign)==='function'){call('unsetAssign',[drop[i]]);}}
 if(typeof(f.setAssign)==='function'){
  call('setAssign',['A','Confirm',function(){var e=document.activeElement;if(e&&e.click){e.click();}}]);
  for(var b in handlers){call('setAssign',[b,b==='B'?'Back':'',handlers[b]]);}
 }
 rep(out);
}
`
