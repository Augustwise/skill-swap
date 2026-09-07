import { ArrowIcon } from "./ArrowIcon";
import { BoltIcon } from "./BoltIcon";
import { ListIcon } from "./ListIcon";
import { PeopleIcon } from "./PeopleIcon";
import { ProfileIcon } from "./ProfileIcon";
import { SproutIcon } from "./SproutIcon";
import type { IconProps } from "./types";

const iconComponents = {
  profile: ProfileIcon,
  list: ListIcon,
  people: PeopleIcon,
  bolt: BoltIcon,
  sprout: SproutIcon,
  arrow: ArrowIcon,
};

export type IconName = keyof typeof iconComponents;

type IconComponentProps = IconProps & {
  name: IconName;
};

export function Icon({ name, ...props }: IconComponentProps) {
  const IconComponent = iconComponents[name];

  return <IconComponent {...props} />;
}
