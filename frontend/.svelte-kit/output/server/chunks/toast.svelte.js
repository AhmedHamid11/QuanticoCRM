import "clsx";
let toasts = [];
let nextId = 0;
function getToasts() {
  return toasts;
}
function addToast(message, type, duration = 3e3) {
  const id = nextId++;
  toasts = [...toasts, { id, message, type }];
  setTimeout(
    () => {
      toasts = toasts.filter((t) => t.id !== id);
    },
    duration
  );
}
const toast = {
  success: (message) => addToast(message, "success"),
  error: (message) => addToast(message, "error", 5e3),
  info: (message) => addToast(message, "info")
};
export {
  getToasts as g,
  toast as t
};
