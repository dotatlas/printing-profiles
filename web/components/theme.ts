// themeKey is the localStorage key that keeps the theme choice of the visitor.
export const themeKey = "theme";

// themeScript runs in <head> before the first paint. It applies a saved "light" or "dark" choice and ignores other values.
export const themeScript = `try{var t=localStorage.getItem("${themeKey}");if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch(e){}`;
