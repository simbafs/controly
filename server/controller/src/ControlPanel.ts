import { Controller } from 'controly-sdk';

interface Command {
  name: string;
  label: string;
  type: 'button' | 'text' | 'number' | 'select' | 'checkbox';
  default?: string | number | boolean;
  regex?: string;
  min?: number;
  max?: number;
  step?: number;
  options?: { label: string; value: string | number }[];
}

export function setupControlPanel(element: HTMLDivElement, displayId: string, commandList: Command[], controller: Controller) {
  element.innerHTML = '<h2>Control Panel for ' + displayId + '</h2>';

  commandList.forEach(command => {
    const controlWrapper = document.createElement('div');
    controlWrapper.className = 'control-item';

    const label = document.createElement('label');
    label.textContent = command.label;
    controlWrapper.appendChild(label);

    let inputElement: HTMLInputElement | HTMLButtonElement | HTMLSelectElement;

    switch (command.type) {
      case 'button':
        inputElement = document.createElement('button');
        inputElement.textContent = command.label;
        inputElement.onclick = () => {
          controller.sendCommand(displayId, { name: command.name });
          console.log(`Sent command: ${command.name} to ${displayId}`);
        };
        break;
      case 'text':
        inputElement = document.createElement('input');
        inputElement.type = 'text';
        inputElement.placeholder = `Enter ${command.label}`;
        if (command.default !== undefined) {
          inputElement.value = String(command.default);
        }
        inputElement.onchange = (e) => {
          const value = (e.target as HTMLInputElement).value;
          if (command.regex && !new RegExp(command.regex).test(value)) {
            alert(`Invalid input for ${command.label}. Please match the required format.`);
            return;
          }
          controller.sendCommand(displayId, { name: command.name, args: { value } });
          console.log(`Sent command: ${command.name} with value: ${value} to ${displayId}`);
        };
        break;
      case 'number':
        inputElement = document.createElement('input');
        inputElement.type = 'number';
        if (command.default !== undefined) {
          inputElement.value = String(command.default);
        }
        if (command.min !== undefined) {
          inputElement.min = String(command.min);
        }
        if (command.max !== undefined) {
          inputElement.max = String(command.max);
        }
        if (command.step !== undefined) {
          inputElement.step = String(command.step);
        }
        inputElement.onchange = (e) => {
          const value = Number((e.target as HTMLInputElement).value);
          controller.sendCommand(displayId, { name: command.name, args: { value } });
          console.log(`Sent command: ${command.name} with value: ${value} to ${displayId}`);
        };
        break;
      case 'select':
        inputElement = document.createElement('select');
        if (command.options) {
          command.options.forEach(option => {
            const optionElement = document.createElement('option');
            optionElement.value = String(option.value);
            optionElement.textContent = option.label;
            inputElement.appendChild(optionElement);
          });
        }
        if (command.default !== undefined) {
          inputElement.value = String(command.default);
        }
        inputElement.onchange = (e) => {
          const value = (e.target as HTMLSelectElement).value;
          controller.sendCommand(displayId, { name: command.name, args: { value } });
          console.log(`Sent command: ${command.name} with value: ${value} to ${displayId}`);
        };
        break;
      case 'checkbox':
        inputElement = document.createElement('input');
        inputElement.type = 'checkbox';
        if (command.default !== undefined) {
          inputElement.checked = Boolean(command.default);
        }
        inputElement.onchange = (e) => {
          const value = (e.target as HTMLInputElement).checked;
          controller.sendCommand(displayId, { name: command.name, args: { value } });
          console.log(`Sent command: ${command.name} with value: ${value} to ${displayId}`);
        };
        break;
      default:
        console.warn(`Unknown command type: ${command.type}`);
        return;
    }

    controlWrapper.appendChild(inputElement);
    element.appendChild(controlWrapper);
  });
}